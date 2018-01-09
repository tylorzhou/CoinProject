package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type Config struct {
	ApiKey, ApiSecret            string
	Emailfrom, Emailpwd, Emailto string
}

var config Config

func init() {
	config.InitFromfile()
}

func (c *Config) InitFromfile() {

	data, err := ioutil.ReadFile("./config/config.json")
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	json.Unmarshal(data, c)
}
