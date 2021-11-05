package main

import (
	"bufio"
	"flag"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"github.com/stianeikeland/go-rpio/v4"
	"go.bug.st/serial"
)

var (
	list       = flag.String("list", "list.txt", "RFID list")
	rfidDevice = flag.String("port", "/dev/ttyUSB0", "reader device")

	OpenPin  rpio.Pin = rpio.Pin(22)
	ClosePin rpio.Pin = rpio.Pin(27)

	latestTimestamp time.Time

	userList map[string]string
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	log.SetLevel(log.DebugLevel)
}

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
			s := strings.ReplaceAll(string(res), "\x03", "")
			s = strings.ReplaceAll(s, "\x02", "")
			c <- s
		}
	}()

	return c
}

func readUserList() (map[string]string, error) {
	users := map[string]string{}
	bytes, err := ioutil.ReadFile(*list)
	if err != nil {
		return users, err
	}
	lines := strings.Split(string(bytes), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 1 {
			users[fields[0]] = strings.Join(fields[1:], " ")
		}
	}

	return users, nil
}

func initUserList() error {
	users, err := readUserList()
	if err != nil {
		return err
	}
	userList = users

	// Set up file listener for file so we can reload the userList when it's edited
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// We need to listen for directory events, as some editors delete the file before rewriting, and listening for
	// changes on the file itself stops when a delete event is received
	absPathToList, err := filepath.Abs(*list)
	if err != nil {
		return err
	}
	dirPathContainingList := filepath.Dir(absPathToList)
	err = watcher.Add(dirPathContainingList)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case fsEvent, ok := <-watcher.Events:
				if !ok {
					return
				}
				if fsEvent.Op&fsnotify.Write != fsnotify.Write || fsEvent.Name != absPathToList {
					continue
				}
				log.WithFields(log.Fields{
					"list":             *list,
					"filesystem event": fsEvent,
				}).Info("received filesystem event for user list, reloading user list")

				users, err := readUserList()
				if err != nil {
					log.WithField("err", err).Error("Failed to reload userList")
					continue
				}
				userList = users
				log.Debugf("Reloaded userList; Found %d users", len(userList))
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error(err)
			}
		}
	}()

	return nil
}

// If token only contains 0 and/or F's, its not a valid token
func isValidToken(token string) bool {
	token = strings.ReplaceAll(token, "F", "")
	token = strings.ReplaceAll(token, "0", "")
	return len(token) > 0
}

func unlockDoor() {
	OpenPin.High()
	time.Sleep(1 * time.Second)
	OpenPin.Low()
}

func main() {
	flag.Parse()

	log.Info("Starting sphincter rfid token...")
	log.Debug("Opening GPIO")
	err := rpio.Open()
	if err != nil {
		log.Fatal(err)
	}
	OpenPin.Output()
	ClosePin.Output()

	log.Debug("Reading list.txt")
	err = initUserList()
	if err != nil {
		log.Fatal(err)
	}
	log.Debugf("Found %d users", len(userList))

	log.Debug("Connecting to Serial")
	mode := &serial.Mode{
		BaudRate: 9600,
	}
	port, err := serial.Open(*rfidDevice, mode)
	if err != nil {
		log.Fatal(err)
	}
	log.Info("Initialized!")

	for readToken := range getRFIDToken(&port) {
		if time.Since(latestTimestamp) < 5*time.Second {
			log.Warn("Triggered too fast; skipped unlock")
			continue
		}
		log.WithField("readToken", readToken).Info("read token")

		username, ok := userList[readToken]
		if ok {
			latestTimestamp = time.Now()
			log.WithFields(log.Fields{
				"username": username,
				"token":    readToken,
			}).Info("Found valid token, unlocking door")
			unlockDoor()
		} else {
			if isValidToken(readToken) {
				log.WithField("token", readToken).Warn("Could not find user to token")
			}
		}
	}
}
