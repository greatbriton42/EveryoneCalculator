package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

var upgrader = websocket.Upgrader{} // use default options
var broadcast = make(chan []byte)
var clients = make(map[*websocket.Conn]bool)
var mutex = &sync.Mutex{}

type Message struct {
	Name       string
	Expression string
}

func compute(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	mutex.Lock()
	clients[c] = true
	mutex.Unlock()

	for {
		var message Message
		err := c.ReadJSON(&message)
		if err != nil {
			log.Println("read:", err)
			break
		}

		first, operator, second, err := parseExpression(message.Expression)
		if err != nil {
			log.Println("parse:", err)
			break
		}

		result, err := calculateExpression(first, second, operator)
		if err != nil {
			log.Println("calculate:", err)
			break
		}

		messageToWrite := fmt.Sprintf("%s: %.2f %s %.2f = %.2f", message.Name, first, operator, second, result)
		broadcast <- []byte(messageToWrite)
	}
}

func broadCastMessages() {
	for {
		message := <-broadcast
		for c := range clients {
			err := c.WriteMessage(websocket.TextMessage, []byte(message))
			if err != nil {
				log.Println("write:", err)
				break
			}
		}

	}

}

func home(w http.ResponseWriter, r *http.Request) {
	homeTemplate.Execute(w, "ws://"+r.Host+"/compute")
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/compute", compute)
	http.HandleFunc("/", home)
	go broadCastMessages()
	fmt.Println("Listening...")
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func parseExpression(expression string) (float64, string, float64, error) {
	if len(expression) == 0 {
		return 0, "", 0, errors.New("no expression to evaluate")
	}

	result := splitExpressionWithDelimiter(expression)

	if len(result) != 3 {
		return 0, "", 0, errors.New("invalid expression")
	}

	processedFirst, err := strconv.ParseFloat(result[0], 64)
	if err != nil {
		return 0, "", 0, errors.New("invalid first number")
	}

	processedSecond, err := strconv.ParseFloat(result[2], 64)
	if err != nil {
		return 0, "", 0, errors.New("invalid second number")
	}

	processedOperator := result[1]

	return processedFirst, processedOperator, processedSecond, nil

}

func splitExpressionWithDelimiter(s string) []string {
	result := []string{}
	start := 0

	for i := 0; i < len(s); i++ {
		if strings.HasPrefix(s[i:], "+") ||
			strings.HasPrefix(s[i:], "-") ||
			strings.HasPrefix(s[i:], "*") ||
			strings.HasPrefix(s[i:], "/") {
			result = append(result, s[start:i])
			result = append(result, s[i:i+1])
			start = i + 1
		}
	}

	result = append(result, s[start:])
	return result
}

func calculateExpression(first float64, second float64, operator string) (float64, error) {
	switch operator {
	case "+":
		return first + second, nil
	case "-":
		return first - second, nil
	case "*":
		return first * second, nil
	case "/":
		return first / second, nil
	default:
		return 0, errors.New("invalid operator")
	}
}

var homeTemplate, _ = template.ParseFiles("homeTemplate.html")
