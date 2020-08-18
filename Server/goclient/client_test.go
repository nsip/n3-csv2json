package goclient

import (
	"io/ioutil"
	"testing"
)

func TestDO(t *testing.T) {
	data, err := ioutil.ReadFile("../../data/Modules.csv")
	failOnErr("%v: ", err)
	str, err := DO(
		"./config.toml",
		"CSV2JSON",
		&Args{
			Data:   data,
			ToNATS: false,
		})
	fPln(str)
	fPln(err)
}
