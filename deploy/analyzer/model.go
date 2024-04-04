package main

type LogEntity struct {
	Level           string  `json:"level"`
	Layer           string  `json:"layer"`
	Id              int     `json:"id"`
	Stop            bool    `json:"stop"`
	Attempts        int     `json:"attempts"`
	Rtt             int64   `json:"rtt"`
	Srtt            float64 `json:"srtt"`
	Rttvar          float64 `json:"rttvar"`
	Rto             float64 `json:"rto"`
	Time            string  `json:"time"`
	WaitingListSize int     `json:"waiting_list_size"`
	Ack             bool    `json:"ack"`
	AckSize         int     `json:"ack_size"`
	TTL             int     `json:"ttl"`
	Message         string  `json:"message"`
}
