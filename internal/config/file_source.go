// Licensed to the LF AI & Data foundation under one
// or more contributor license agreements. See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership. The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/milvus-io/milvus/internal/log"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type FileSource struct {
	sync.RWMutex
	file    string
	configs map[string]string

	configRefresher refresher
}

func NewFileSource(fileInfo *FileInfo) *FileSource {
	fs := &FileSource{
		file:    fileInfo.Filepath,
		configs: make(map[string]string),
	}
	fs.configRefresher = newRefresher(fileInfo.RefreshInterval, fs.loadFromFile)
	return fs
}

// GetConfigurationByKey implements ConfigSource
func (fs *FileSource) GetConfigurationByKey(key string) (string, error) {
	fs.RLock()
	v, ok := fs.configs[key]
	fs.RUnlock()
	if !ok {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return v, nil
}

// GetConfigurations implements ConfigSource
func (fs *FileSource) GetConfigurations() (map[string]string, error) {
	configMap := make(map[string]string)

	err := fs.loadFromFile()
	if err != nil {
		return nil, err
	}

	fs.configRefresher.start(fs.GetSourceName())

	fs.RLock()
	for k, v := range fs.configs {
		configMap[k] = v
	}
	fs.RUnlock()
	return configMap, nil
}

// GetPriority implements ConfigSource
func (fs *FileSource) GetPriority() int {
	return LowPriority
}

// GetSourceName implements ConfigSource
func (fs *FileSource) GetSourceName() string {
	return "FileSource"
}

func (fs *FileSource) Close() {
	fs.configRefresher.stop()
}

func (fs *FileSource) SetEventHandler(eh EventHandler) {
	fs.configRefresher.eh = eh
}
func (fs *FileSource) loadFromFile() error {
	yamlReader := viper.New()
	configFile := fs.file
	if _, err := os.Stat(configFile); err != nil {
		return errors.New("cannot access config file: " + configFile)
	}

	yamlReader.SetConfigFile(configFile)
	if err := yamlReader.ReadInConfig(); err != nil {
		log.Warn("Read config failed", zap.Error(err))
		return err
	}

	newConfig := make(map[string]string)
	for _, key := range yamlReader.AllKeys() {
		val := yamlReader.Get(key)
		str, err := cast.ToStringE(val)
		if err != nil {
			switch val := val.(type) {
			case []interface{}:
				str = str[:0]
				for _, v := range val {
					ss, err := cast.ToStringE(v)
					if err != nil {
						log.Warn("cast to string failed", zap.Any("value", v))
					}
					if str == "" {
						str = ss
					} else {
						str = str + "," + ss
					}
				}

			default:
				log.Warn("val is not a slice", zap.Any("value", val))
				continue
			}
		}
		newConfig[key] = str
		newConfig[formatKey(key)] = str
	}

	fs.Lock()
	defer fs.Unlock()
	err := fs.configRefresher.fireEvents(fs.GetSourceName(), fs.configs, newConfig)
	if err != nil {
		return err
	}
	fs.configs = newConfig

	return nil
}
