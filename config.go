package main

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"sync"
)

// config 是一个全局的配置信息实例 项目运行只读取一次 是一个单例
var config *Config
var once sync.Once

// GetConfig 调用该方法会实例化conf 项目运行会读取一次配置文件 确保不会有多余的读取损耗
func GetConfig() *Config {
	once.Do(func() {
		config = new(Config)
		yamlFile, err := ioutil.ReadFile("config.yaml")
		if err != nil {
			panic(err)
		}
		err = yaml.Unmarshal(yamlFile, config)
		if err != nil {
			//读取配置文件失败,停止执行
			panic("read config file error:" + err.Error())
		}
	})
	return config
}
