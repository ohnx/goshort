package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	"io"
	"net/http"
	"os"
)

var client *redis.Client = nil
var auth_key = os.Getenv("GOS_AUTHKEY")
var server = os.Getenv("GOS_SERVER")

func SetupRedisNewClient() {
	client = redis.NewClient(&redis.Options{
		Addr:     server,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	pong, err := client.Ping().Result()

	if err != nil {
		fmt.Println("Failed connection to redis server!")
	} else {
		fmt.Println("Established connection to redis server! " + pong)
	}
}

type catchAll struct{}

func (*catchAll) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST")

	if r.URL.String() == "/" {
		http.Redirect(w, r, "https://masonx.ca/hub", http.StatusMovedPermanently)
	} else if r.URL.String() == "/favicon.ico" {
		io.WriteString(w, "No.")
	} else if r.URL.String() == "/add" && r.Header.Get("Content-Type") == "application/json" {
		decoder := json.NewDecoder(r.Body)

		var f interface{}
		err := decoder.Decode(&f)
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "Failed to parse JSON.")
			defer r.Body.Close()
			return
		}
		m := f.(map[string]interface{})

		var longurl string
		var shorturl string
		var authcode string = "HAHAH"
		i := 0 // prevent looping too much in the reply, a possible DoS

		for k, v := range m {
			switch vv := v.(type) {
			case string:
				if k == "longurl" {
					longurl = vv
				} else if k == "shorturl" {
					shorturl = vv
				} else if k == "authcode" {
					authcode = vv
				}
			}
			i++
			if i > 4 {
				break
			}
		}

		if auth_key == authcode {
			_, err := client.Get(shorturl).Result()
			if err == redis.Nil {
				err := client.Set(shorturl, longurl, 0).Err()

				if err != nil {
					io.WriteString(w, "DB error!")
					fmt.Println("DB error. Trying to reconnect...")
					SetupRedisNewClient()
				} else {
					io.WriteString(w, "Added!")
				}
			} else if err != nil {
				io.WriteString(w, "DB error!")
				fmt.Println("DB error. Trying to reconnect...")
				SetupRedisNewClient()
			} else {
				io.WriteString(w, "URL already exists!")
			}
		} else {
			io.WriteString(w, "Failed auth.\n")
		}
	} else {
		key := r.URL.String()[1:len(r.URL.String())] // remove the leading slash
		val, err := client.Get(key).Result()

		if err == redis.Nil {
			io.WriteString(w, "No such URL `"+key+"`\n")
		} else if err != nil {
			io.WriteString(w, "DB error!")
			fmt.Println("DB error. Trying to reconnect...")
			SetupRedisNewClient()
		} else {
			http.Redirect(w, r, val, http.StatusMovedPermanently)
		}
	}
	defer r.Body.Close()
}

func main() {
	server := http.Server{
		Addr:    ":8080",
		Handler: &catchAll{},
	}

	SetupRedisNewClient()
	server.ListenAndServe()
}
