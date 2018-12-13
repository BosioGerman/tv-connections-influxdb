package main

import (
	"fmt"
	"github.com/influxdata/influxdb/client/v2"
	"github.com/labstack/gommon/log"
	"os"
	"strconv"
	"strings"
	"time"
	"gopkg.in/resty.v1"
	"encoding/json"
)
type Tvsessionitem struct {
	ID               string    `json:"id"`
	Userid           string    `json:"userid"`
	Username         string    `json:"username"`
	Deviceid         string    `json:"deviceid"`
	StartDate        time.Time `json:"start_date"`
	EndDate          time.Time `json:"end_date"`
	Fee              float64   `json:"fee"`
	Currency         string    `json:"currency"`
	BillingState     string    `json:"billing_state"`
	ContactID        string    `json:"contact_id,omitempty"`
	SessionCode      string    `json:"session_code,omitempty"`
	SessionCreatedAt time.Time `json:"session_created_at,omitempty"`
	ValidUntil       time.Time `json:"valid_until,omitempty"`
	AssignedUserid   string    `json:"assigned_userid,omitempty"`
	AssignedAt       time.Time `json:"assigned_at,omitempty"`
	EndCustomer      struct {
		Name string `json:"name"`
	} `json:"end_customer,omitempty"`
}

type Tvsession struct {
	Records []Tvsessionitem `json:"records"`
	RecordsRemaining int `json:"records_remaining"`
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func logIfError(err error) {
	if err != nil {
		log.Error(err)
	}
}
func getConnections(host, path, token string) (list []Tvsessionitem) {
	//Get all teamviewer connections
	resty.SetHostURL(host)
	resp, err := resty.R().
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", token)).
		Get(path)

	logIfError(err)

	jsonResponse := resp.String()
	var sessions Tvsession
	json.Unmarshal([]byte(jsonResponse), &sessions)

	list = make([]Tvsessionitem, 0)

	//Filter connections that do not have end_date, that is, active connections
	for _, value := range sessions.Records {
		if value.EndDate.IsZero() {
			list = append(list, value)
		} else {
			return
		}
	}

	return
}

func main() {
	protocol := getEnv("INFLUXDB_PROTOCOL", "http")
	host := getEnv("INFLUXDB_HOST", "localhost")
	influxdbPort := getEnv("INFLUXDB_PORT", "8084")

	port, err := strconv.Atoi(influxdbPort)
	if err != nil {
		port = 8084
	}

	database := getEnv("INFLUXDB_DATABASENAME", "tv")
	user := getEnv("INFLUXDB_USERNAME", "")
	password := getEnv("INFLUXDB_USER_PASSWORD","")
	token := getEnv("TV_TOKEN", "")
	checkInterval := getEnv("CHECK_INTERVAL", "60")

	interval, err := strconv.Atoi(checkInterval)
	if err != nil {
		interval = 60
	}
	tvhost := os.Getenv("TV_HOST")
	tvpath := os.Getenv("TV_PATH")
	fmt.Printf("InfluxDB Protocol: %s\r\nInfluxDB Host: %s\r\nInfluxDB port: %s\r\nInfluxDB Database: %s\r\nInfluxDB User: %s\r\nInfluxDB Password: %s\r\nTeamviewer Token: %s\r\nCheck Interval: %s\r\nTeamviewer Host: %s\r\nTeamviewer Path: %s\r\n ",protocol, host, port, database, user, password, token, interval, tvhost, tvpath)

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	quit := make(chan struct{})
	var cli client.Client
	if strings.ToLower(protocol) == "http" {
		cli, err = client.NewHTTPClient(client.HTTPConfig{
			Addr:     fmt.Sprintf("%s:%d", host, port),
			Username: user,
			Password: password,
		})
		logIfError(err)

	} else {
		cli, err = client.NewUDPClient(client.UDPConfig{
			Addr: fmt.Sprintf("%s:%d", host, port)})
		logIfError(err)
	}
	defer cli.Close()


	for {
		select {
		case <- ticker.C:
			//Get teamviewer active connections
			connections := getConnections(tvhost, tvpath, token)

			// Create a new point batch
			bp, err := client.NewBatchPoints(client.BatchPointsConfig{
				Database:  database,
				Precision: "s",
			})
			logIfError(err)

			// Create a point and add to batch
			tags := map[string]string{"type": "opened"}
			fields := map[string]interface{}{
				"value": len(connections),
			}

			pt, err := client.NewPoint("sessions", tags, fields)
			logIfError(err)
			bp.AddPoint(pt)

			// Write the batch
			err = cli.Write(bp)
			logIfError(err)

		case <- quit:
			ticker.Stop()
			return
		}
	}
}
