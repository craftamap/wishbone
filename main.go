package main

import (
	"bufio"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/stianeikeland/go-rpio"
	"go.bug.st/serial"
)

var (
	OpenPin  rpio.Pin = rpio.Pin(22)
	ClosePin rpio.Pin = rpio.Pin(27)
)

func getRFIDToken(port *serial.Port) chan string {
	c := make(chan string)

	go func() {
		for {
			rd := bufio.NewReader(*port)
			res, err := rd.ReadBytes('\x03')
			if err != nil {
				// If there was an error while reading from the port,
				// panic so daemon will restart
				panic(err)
			}
			s := strings.Replace(string(res), "\x03", "", -1)
			s = strings.Replace(s, "\x02", "", -1)
			c <- s
		}
	}()

	return c
}

func parseUserList() (map[string]string, error) {
	users := map[string]string{}
	bytes, err := ioutil.ReadFile("list.txt")
	if err != nil {
		return users, err
	}
	lines := strings.Split(string(bytes), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			users[fields[0]] = fields[1]
		}
	}

	return users, nil
}

func main() {
	log.Println(" :: Starting sphincter rfid token...")
	log.Println(" :::: Opening GPIO")
	err := rpio.Open()
	if err != nil {
		log.Fatal(err)
	}
	OpenPin.Output()
	ClosePin.Output()

	log.Println(" :::: Reading list.txt")
	users, err := parseUserList()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf(" :::: Found %d users \n", len(users))
	// log.Printf("%v\n", users)

	log.Println(" :::: Connecting to Serial")
	mode := &serial.Mode{
		BaudRate: 9600,
	}
	port, err := serial.Open("/dev/ttyUSB0", mode)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(" :: Initialized!")

	for msg := range getRFIDToken(&port) {
		username, ok := users[msg]
		if ok {
			log.Printf("Hello %s %s", msg, username)
			OpenPin.High()
			time.Sleep(1 * time.Second)
			OpenPin.Low()
		}
	}
}
