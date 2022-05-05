package main

import (
	"math/rand"
	"time"
)


const (
	TYPE_USER = 1
	TYPE_SERV = 2
)

type App struct {

}



type DataRecord struct {
	Id 	int
	Name string
	Type int	//1 user 2 service
	Key string
	ServiceName string
}

var Data map[int]DataRecord

func getRandIntNum(max int) int{
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max)
}

func (app *App)GetById(appId int)DataRecord{
	return Data[appId]
}

func (app *App) loadAppData(){
	Data = make(map[int]DataRecord)
	dataRecord := DataRecord{
		Id:1,
		Name : "testMqtt",
		Type : 1,
		Key : "123456",
	}
	Data[1] = dataRecord

	dataRecord = DataRecord{
		Id:2,
		Name : "testMqtt",
		Type : 2,
		Key : "123456",
		ServiceName:"ucenter",
	}
	Data[2] = dataRecord
}

func  NewApp() *App{
	app := new (App)
	app.loadAppData()
	return app
}