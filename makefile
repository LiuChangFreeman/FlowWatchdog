all: main

main:main.go util.go
	go build -o main main.go util.go

clean:
	rm -rf main