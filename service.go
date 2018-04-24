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

// Цена на момент времени
type PriceAtMoment struct {
	Price float64
	Time  time.Time
}

// Новый тип для сериализации с заданным количеством знаков после запятой
type Number float64
func (n Number) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%.4f", n)), nil
}

// Пара валют со всеми ценами за 10 минут и средним
type Pair struct {
	AvgP      Number
	allPrices []PriceAtMoment
}
var pairs map[string]Pair
var pairsMutex sync.RWMutex

var apiUrl = "https://wex.nz/api/3/"

// структура для парсинга ответа сервера
type serverInfoResponse struct {
	ServerTime int                    `json:"server_time"`
	Pairs      map[string]interface{} `json:"pairs"`
}

// свой клиент для установки таймаута
var myClient = &http.Client{Timeout: 5 * time.Second}

// срока содержащая все пары для запроса
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
			pair.allPrices = append(pair.allPrices, PriceAtMoment{price, time.Now()})

			// вычисляем и заменяем среднее за 10 минут
			if len(pair.allPrices) > 0 {
				sum, num := 0.0, 0
				newPrices := make([]PriceAtMoment, 0)
				for _, p := range pair.allPrices {
					if time.Since(p.Time).Minutes() < 10 {
						sum += p.Price
						num ++
						// удаляем старые цены из памяти
						newPrices = append(newPrices, p)
					}
				}
				pair.allPrices = newPrices
				pair.AvgP = Number(sum / float64(num))
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
	rtr.HandleFunc("/ticker/{pair}", getSinglePairHandler).Methods("GET")
	rtr.HandleFunc("/", getAllPairsHandler).Methods("GET")
	http.Handle("/", rtr)
	log.Println("Listening...")
	http.ListenAndServe(":3000", nil)
}

func getSinglePairHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	pairsMutex.RLock()
	pair, ok := pairs[params["pair"]]
	pairsMutex.RUnlock()
	if !ok {
		w.Write([]byte("Такой пары не найдено, или котировки еще не загружены"))
		return
	}
	s := fmt.Sprintf("{'price': %v}", pair.AvgP)
	w.Write([]byte(s))
}


func getAllPairsHandler(w http.ResponseWriter, r *http.Request) {
	pairsMutex.RLock()
	jsonInfo, _ := json.Marshal(pairs)
	pairsMutex.RUnlock()

	s := fmt.Sprintf("%v", string(jsonInfo))
	w.Write([]byte(s))
}
