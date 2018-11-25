package models

import (
	"github.com/inwinstack/kaoliang/pkg/utils"
	"github.com/olivere/elastic"
)

var elsClient *elastic.Client

func SetElasticsearch() {
	var err error
	elsClient, err = elastic.NewClient(
		elastic.SetURL(utils.GetEnv("ELS_URL", "http://localhost:9200")),
	)
	if err != nil {
		panic(err)
	}
}

func GetElasticsearch() *elastic.Client {
	return elsClient
}
