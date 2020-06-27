package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/stianeikeland/go-rpio"
	"go.bug.st/serial"
)

func getRFIDToken(port *serial.Port, c chan string) {
	for {
		rd := bufio.NewReader(*port)
		res, err := rd.ReadBytes('\x03')
		if err != nil {
			return
		}
		s := strings.Replace(string(res), "\x03", "", -1)
		s = strings.Replace(s, "\x02", "", -1)
		c <- s
	}
}

var users map[string]string = map[string]string{}
var OpenPin rpio.Pin = rpio.Pin(22)
var ClosePin rpio.Pin = rpio.Pin(27)

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
	bytes, err := ioutil.ReadFile("list.txt")
	if err != nil {
		log.Fatal(err)
	}
	lines := strings.Split(string(bytes), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			users[fields[0]] = fields[1]
		}
	}
	fmt.Printf(" :::: Found %d users \n", len(users))
	// fmt.Printf("%v\n", users)
	fmt.Println(" :::: Connecting to Serial")
	mode := &serial.Mode{
		BaudRate: 9600,
	}
	port, err := serial.Open("/dev/ttyUSB0", mode)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(" :: Initialized!")
	c := make(chan string)
	go getRFIDToken(&port, c)
	for msg := range c {
		username, ok := users[msg]
		if ok {
			log.Printf("%s: Hello %s %s", time.Now(), msg, username)
			OpenPin.High()
			time.Sleep(1 * time.Second)
			OpenPin.Low()
		}
	}
}
