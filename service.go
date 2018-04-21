package main

import (
	"net/http"
	"time"
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
	"fmt"
	"io/ioutil"
	"sync"
)

type PriceAtMoment struct {
	Price float64
	Time  time.Time
}

type Pair struct {
	AveragePrice float64
	AllPrices []PriceAtMoment
}

var apiUrl = "https://wex.nz/api/3/"

var pairsMutex sync.RWMutex
var pairs map[string]Pair

type serverInfoResponse struct {
	ServerTime int                    `json:"server_time"`
	Pairs      map[string]interface{} `json:"pairs"`
}

var myClient = &http.Client{Timeout: 10 * time.Second}

var pairString string

func populatePairs() {
	// Получить с сервера все возможные пары
	r, err := myClient.Get(apiUrl + "info")
	if err != nil {
		panic(err)
	}
	defer r.Body.Close()

	// Прочитать JSON ответ
	m := serverInfoResponse{}
	bodyBytes, _ := ioutil.ReadAll(r.Body)
	err = json.Unmarshal(bodyBytes, &m)
	if err != nil {
		panic(err)
	}

	// заполнить массив пар
	pairString = ""
	pairs = make(map[string]Pair, len(m.Pairs))
	i := 0
	for key := range m.Pairs {
		pairs[key] = Pair{}
		if i > 0 {
			pairString += "-" + key
		} else {
			pairString += key
		}
		i ++
	}

	// Начать заполнять пары
	go pollPairs()
}

func pollPairs() {
	for {
		// Получить с сервера котировки пар
		r, err := myClient.Get(apiUrl + "ticker/" + pairString)
		if err != nil {
			panic(err)
		}
		// Прочитать JSON ответ
		var m map[string]map[string]float64
		bodyBytes, _ := ioutil.ReadAll(r.Body)

		// закрываем соединение
		r.Body.Close()

		err = json.Unmarshal(bodyBytes, &m)
		if err != nil {
			panic(err)
		}

		// обновить котировки в базе
		for key, value := range m {
			pairsMutex.RLock()
			pair, ok := pairs[key]
			pairsMutex.RUnlock()
			if !ok {
				fmt.Println("Такой котировки не найдено")
			}
			price := value["last"]
			pair.AllPrices = append(pair.AllPrices, PriceAtMoment{price, time.Now()})

			// вычисляем и заменяем среднее за 10 минут
			if len(pair.AllPrices) > 0 {
				sum, num := 0.0, 0
				newPrices := make([]PriceAtMoment, 0)
				for _, p := range pair.AllPrices {
					if time.Since(p.Time).Minutes() < 10 {
						sum += p.Price
						num ++
						// удаляем старые цены из памяти
						newPrices = append(newPrices, p)
					}
				}
				pair.AllPrices = newPrices
				pair.AveragePrice = sum / float64(num)
			}

			// кладем обратно пару в хранилище
			pairsMutex.Lock()
			pairs[key] = pair
			pairsMutex.Unlock()
		}

		// ждем следующего опроса
		time.Sleep(2 * time.Second)
	}
}

func main() {
	// Начинаем заполнять пары с биржы в память
	go populatePairs()

	// Поднимаем веб-сервер для обслуживания API
	rtr := mux.NewRouter()
	rtr.HandleFunc("/ticker/{pair}", handler).Methods("GET")
	http.Handle("/", rtr)
	log.Println("Listening...")
	http.ListenAndServe(":3000", nil)
}

func handler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	pairsMutex.RLock()
	pair, ok := pairs[params["pair"]]
	pairsMutex.RUnlock()
	if !ok {
		w.Write([]byte("Такой пары не найдено, или котировки еще не загружены"))
		return
	}
	s := fmt.Sprintf("{'price': %v}", pair.AveragePrice)
	w.Write([]byte(s))
}
