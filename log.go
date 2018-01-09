package main

import (
	"io"
	"log"
	"os"
)

var (
	Trace   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
	History *log.Logger
)

//Initfile init log
func Initfile(
	traceHandle io.Writer,
	infoHandle io.Writer,
	warningHandle io.Writer,
	errorHandle io.Writer,
	history io.Writer) {

	Trace = log.New(traceHandle,
		"Trace: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Info = log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Warning = log.New(warningHandle,
		"WARNING: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Error = log.New(errorHandle,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	History = log.New(history,
		"",
		log.Ldate|log.Ltime)
}

//InitLog init log
func InitLog() {
	os.MkdirAll("./log", os.ModePerm)
	file, err := os.OpenFile("./log/Tracefile.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("cannot generate log file Tracefile")
	}

	file1, err := os.OpenFile("./log/Infofile.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("cannot generate log file Infofile")
	}

	file2, err := os.OpenFile("./log/Warningfile.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("cannot generate log file Warningfile")
	}

	file3, err := os.OpenFile("./log/Errorfile.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("cannot generate log file Errorfile")
	}

	file4, err := os.OpenFile("./log/History.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("cannot generate log file Errorfile")
	}

	Initfile(file, file1, file2, file3, file4)

}
