FROM scratch
ADD ./bin /home
WORKDIR /home
CMD ["/home/main"]