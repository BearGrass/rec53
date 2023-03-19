package logger

import (
	"log"

	"go.uber.org/zap"
)

var Rec53Log *zap.Logger

func Init() {
	var err error
	Rec53Log, err = zap.NewDevelopment()
	if err != nil {
		log.Fatal("create zap error: ", err)
	}
}
