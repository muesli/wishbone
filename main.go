package main

import (
	"bufio"
	"flag"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
	"go.bug.st/serial"
)

var (
	list = flag.String("list", "list.txt", "RFID list")
	port = flag.String("port", "/dev/ttyUSB0", "reader device")

	OpenPin  rpio.Pin = rpio.Pin(22)
	ClosePin rpio.Pin = rpio.Pin(27)

	latestTimestamp time.Time
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

// If token only contains 0 and/or F's, its not a valid token
func isValid(token string) bool {
	token = strings.ReplaceAll(token, "F", "")
	token = strings.ReplaceAll(token, "0", "")
	return len(token) > 0
}

func main() {
	flag.Parse()

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
	port, err := serial.Open(*port, mode)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(" :: Initialized!")

	for msg := range getRFIDToken(&port) {
		if time.Since(latestTimestamp) < 5*time.Second {
			log.Println("Triggered too fast; skipped unlock")
			continue
		}

		username, ok := users[msg]
		if ok {
			latestTimestamp = time.Now()
			log.Printf("Hello %s %s", msg, username)
			OpenPin.High()
			time.Sleep(1 * time.Second)
			OpenPin.Low()
		} else {
			if isValid(msg) {
				log.Printf("Could not find key %s", msg)
			}
		}
	}
}
