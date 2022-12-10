package ronnyd

import (
	"encoding/json"
	"io/ioutil" //it will be used to help us read our config.json file.
)

func ReadConfig() map[string]interface{} {
	//read the config.json file
	file, err := ioutil.ReadFile("config.json")
	if err != nil {
		panic("Error reading config file")
	}
	//unmarshal the file into a map
	var config map[string]interface{}
	err = json.Unmarshal(file, &config)
	if err != nil {
		panic("Unable to parse json in config file")
	}
	return config
}
