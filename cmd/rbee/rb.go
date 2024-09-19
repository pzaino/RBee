// Copyright 2023 Paolo Fabio Zaino
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This command is a simple automation server that listens for commands to move the mouse and click or perform keyboard actions simulating a human user.
package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	cmn "github.com/pzaino/thecrowler/pkg/common"
	"golang.org/x/time/rate"

	"github.com/go-vgo/robotgo"
)

var (
	limiter *rate.Limiter
)

func main() {
	srvPort := flag.String("port", "3000", "port on where to listen for commands")
	srvHost := flag.String("host", "localhost", "host on where to listen for commands")
	sslMode := flag.String("sslmode", "disable", "enable or disable SSL")
	crtFile := flag.String("certfile", "", "path to the SSL certificate file")
	keyFile := flag.String("keyfile", "", "path to the SSL key file")
	rateLmt := flag.String("ratelimit", "10,10", "rate limit in requests per second and burst limit")
	flag.Parse()

	host := *srvHost
	port := *srvPort
	fSSL := *sslMode
	cert := *crtFile
	key := *keyFile
	rtLmt := *rateLmt

	srv := &http.Server{
		Addr: host + ":" + port,

		// ReadHeaderTimeout is the amount of time allowed to read
		// request headers. The connection's read deadline is reset
		// after reading the headers and the Handler can decide what
		// is considered too slow for the body. If ReadHeaderTimeout
		// is zero, the value of ReadTimeout is used. If both are
		// zero, there is no timeout.
		ReadHeaderTimeout: time.Duration(45) * time.Second,

		// ReadTimeout is the maximum duration for reading the entire
		// request, including the body. A zero or negative value means
		// there will be no timeout.
		//
		// Because ReadTimeout does not let Handlers make per-request
		// decisions on each request body's acceptable deadline or
		// upload rate, most users will prefer to use
		// ReadHeaderTimeout. It is valid to use them both.
		ReadTimeout: time.Duration(60) * time.Second,

		// WriteTimeout is the maximum duration before timing out
		// writes of the response. It is reset whenever a new
		// request's header is read. Like ReadTimeout, it does not
		// let Handlers make decisions on a per-request basis.
		// A zero or negative value means there will be no timeout.
		WriteTimeout: time.Duration(45) * time.Second,

		// IdleTimeout is the maximum amount of time to wait for the
		// next request when keep-alive are enabled. If IdleTimeout
		// is zero, the value of ReadTimeout is used. If both are
		// zero, there is no timeout.
		IdleTimeout: time.Duration(45) * time.Second,
	}

	var rl, bl int
	rl, err := strconv.Atoi(strings.Split(rtLmt, ",")[0])
	if err != nil {
		rl = 10
	}
	bl, err = strconv.Atoi(strings.Split(rtLmt, ",")[1])
	if err != nil {
		bl = 10
	}
	limiter = rate.NewLimiter(rate.Limit(rl), bl)

	// Add a handler for the command endpoint
	initAPIv1()

	// Start the server
	log.Printf("Starting server on port %s", port)
	if strings.ToLower(strings.TrimSpace(fSSL)) == "enable" {
		log.Fatal(srv.ListenAndServeTLS(cert, key))
	}
	log.Fatal(srv.ListenAndServe())
}

func initAPIv1() {
	cmdHandlerWithMiddlewares := SecurityHeadersMiddleware(RateLimitMiddleware(http.HandlerFunc(commandHandler)))

	http.Handle("/v1/rb", cmdHandlerWithMiddlewares)
}

// RateLimitMiddleware is a middleware for rate limiting
func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			cmn.DebugMsg(cmn.DbgLvlDebug, "Rate limit exceeded")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeadersMiddleware adds security-related headers to responses
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add various security headers here
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")

		next.ServeHTTP(w, r)
	})
}

func commandHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST method is accepted", http.StatusMethodNotAllowed)
		return
	}

	var cmd Command
	err := json.NewDecoder(r.Body).Decode(&cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = executeCommand(cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte("Command executed"))
	if err != nil {
		log.Println("Error writing response:", err)
	}
}

func executeCommand(cmd Command) error {
	switch cmd.Action {
	case "moveMouse":
		return MouseMove(cmd.X, cmd.Y)
	case "click":
		robotgo.Click()
		return nil
	case "right_click":
		robotgo.Click("right")
		return nil
	case "type":
		return TypeStr(cmd.Value)
	case "keyTap":
		err := robotgo.KeyTap(cmd.Value)
		return err
	default:
		return fmt.Errorf("unknown action: %s", cmd.Action)
	}
}

func pseudoCircularMovement(x, y, steps int, s, v float64) {
	r := float64(getRandInt(16, 50))   // radius between 16 and 66
	p := getRandInt(5, 50)             // micro-pauses between steps
	pi := math.Pi                      // Pi constant for calculation
	clockwise := getRandInt(0, 1) == 0 // Direction of the circle

	// Calculate the starting point relative to x and y
	startX, startY := robotgo.Location()

	// Move to the starting position smoothly to avoid sudden jumps
	robotgo.MoveSmooth(x, y, s, v)
	time.Sleep(time.Duration(p) * time.Millisecond)

	// Adjust the center based on the desired radius and position
	centerX, centerY := x-int(r*math.Cos(pi/2)), y-int(r*math.Sin(pi/2))

	for i := 0; i <= steps; i++ {
		angle := 2 * pi * float64(i) / float64(steps) // Full circle

		// Adjust the angle for clockwise or counterclockwise movement
		if !clockwise {
			angle = -angle
		}

		newX := centerX + int(r*math.Cos(angle))
		newY := centerY + int(r*math.Sin(angle))

		// Move the mouse smoothly to the new coordinates
		robotgo.MoveSmooth(newX, newY, s, v)

		// Simulate human-like movement with a delay
		time.Sleep(time.Duration(p) * time.Millisecond)
	}

	// Optionally, move back to the original position smoothly
	robotgo.MoveSmooth(startX, startY, s, v)
}

// MouseMove introduces small jittery movements during mouse movement
func MouseMove(x, y int) error {
	currentX, currentY := robotgo.Location()
	s := getRandFloat(0.5, 1.5) // speed
	v := getRandFloat(0.5, 1.5) // end velocity
	fmt.Printf("Moving mouse to %d, %d with speed %f and end velocity %f\n", x, y, s, v)

	moveToWithJitter(currentX, currentY, x, y, s, v)
	return nil
}

func moveToWithJitter(startX, startY, endX, endY int, speed, velocity float64) {
	steps := 10
	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps)
		curX := int(float64(startX) + t*float64(endX-startX))
		curY := int(float64(startY) + t*float64(endY-startY))
		curX += getRandInt(-2, 2) // adding jitter
		curY += getRandInt(-2, 2) // adding jitter
		robotgo.MoveSmooth(curX, curY, speed, velocity)
		time.Sleep(time.Duration(getRandInt(10, 50)) * time.Millisecond)
	}
	if getRandInt(0, 1) > 0 {
		pseudoCircularMovement(endX, endY, 10, speed, velocity)
	}
	robotgo.MoveSmooth(endX, endY, speed, velocity)
}

// TypeStr provides enhanced typing function to simulate human-like behavior with errors and corrections
func TypeStr(str string) error {
	for _, c := range str {
		if getRandInt(0, 100) < 5 { // 5% chance to simulate a typo
			typo := getRandInt(0, len(str)-1)
			robotgo.TypeStr(string(str[typo]))
			time.Sleep(time.Duration(getRandInt(100, 300)) * time.Millisecond) // simulate pause for correction
			err := robotgo.KeyTap("Backspace")
			if err != nil {
				return err
			}
		}

		robotgo.TypeStr(string(c))
		time.Sleep(time.Duration(getRandInt(50, 300)) * time.Millisecond) // variable typing speed
	}
	return nil
}

// Existing utility functions for randomness
func getRandInt(min, max int) int {
	rangeInt := big.NewInt(int64(max - min + 1))
	n, err := rand.Int(rand.Reader, rangeInt)
	if err != nil {
		return 0
	}
	iRnd := int((*n).Int64())
	return min + iRnd
}

func getRandFloat(min, max float64) float64 {
	rangeInt := big.NewInt(int64((max * 100) - (min * 100) + 1))
	n, err := rand.Int(rand.Reader, rangeInt)
	if err != nil {
		return 0
	}
	fRnd := float64((*n).Int64()) / 100
	return min + fRnd
}
