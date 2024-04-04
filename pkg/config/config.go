package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/radovskyb/watcher"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/forest33/tapir/pkg/logger"
	"github.com/forest33/tapir/pkg/structs"
)

const (
	tagDefault = "default"
)

type Config struct {
	path      string
	data      interface{}
	log       *logger.Logger
	observers []func(interface{})
}

func New(configFileName, configFileDir string, cfg interface{}) (*Config, error) {
	path, ok := os.LookupEnv("TAPIR_CONFIG")
	if !ok {
		if configFileDir != "" {
			path = filepath.Join(configFileDir, configFileName)
		} else {
			ex, err := os.Executable()
			if err != nil {
				return nil, err
			}
			path = filepath.Join(filepath.Dir(ex), configFileName)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	if err = yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if err := Parse(cfg); err != nil {
		return nil, err
	}

	return &Config{
		path:      path,
		data:      cfg,
		observers: make([]func(interface{}), 0, 1),
		log:       logger.NewDefault(),
	}, nil
}

func (c *Config) Update(data interface{}) {
	c.data = data
}

func (c *Config) Save() {
	buf, err := yaml.Marshal(c.data)
	if err != nil {
		log.Printf("failed to marshal config: %v", err)
		return
	}
	if err := os.WriteFile(c.path, buf, 0664); err != nil {
		log.Printf("failed to write config file: %v", err)
	}
}

func (c *Config) GetPath() string {
	return c.path
}

func (c *Config) AddObserver(f func(interface{})) error {
	if len(c.observers) == 0 {
		if err := c.startWatcher(); err != nil {
			return err
		}
	}
	c.observers = append(c.observers, f)
	return nil
}

func (c *Config) startWatcher() error {
	w := watcher.New()
	w.SetMaxEvents(1)
	w.FilterOps(watcher.Write)
	if err := w.Add(c.path); err != nil {
		return err
	}

	go func() {
		if err := w.Start(time.Second); err != nil {
			c.log.Fatalf("failed to start watching config file: %v", err)
			return
		}
		defer w.Close()
	}()

	go func() {
		for {
			select {
			case <-w.Event:
				c.log.Info().Str("path", c.path).Msg("config file changed")
				data, err := os.ReadFile(c.path)
				if err != nil {
					c.log.Error().Err(err).Msg("failed to read config file")
					continue
				}
				if err = yaml.Unmarshal(data, c.data); err != nil {
					c.log.Error().Err(err).Msg("failed to unmarshal config file")
					continue
				}
				if err := Parse(c.data); err != nil {
					c.log.Error().Err(err).Msg("failed to parse config file")
					continue
				}
				for i := range c.observers {
					c.observers[i](c.data)
				}
			case err := <-w.Error:
				c.log.Error().Err(err).Msg("error on watching config file")
			case <-w.Closed:
				return
			}
		}
	}()

	return nil
}

func Parse(target interface{}) error {
	ref := reflect.Indirect(reflect.ValueOf(target))
	for i := 0; i < ref.Type().NumField(); i++ {
		structField := ref.Type().Field(i)
		fieldValue := ref.Field(i)

		if isSet(structField, &fieldValue) {
			continue
		}

		defaultTagValue, defaultTagExists := structField.Tag.Lookup(tagDefault)

		if defaultTagExists {
			if err := setValue(structField, &fieldValue, defaultTagValue); err != nil {
				return err
			}
			continue
		}

		if fieldValue.IsZero() && structField.Type.Kind() != reflect.Bool && structField.Type.Kind() != reflect.Ptr && structField.Type.Kind() != reflect.Slice {
			return fmt.Errorf("required configuration parameter is not specified - %s.%s", ref.Type().Name(), structField.Name)
		}

		if structField.Type.Kind() == reflect.Ptr || structField.Type.Kind() == reflect.Slice {
			if err := setValue(structField, &fieldValue, ""); err != nil {
				return err
			}
		}
	}

	return nil
}

func isSet(structField reflect.StructField, field *reflect.Value) bool {
	if structField.Type.Kind() == reflect.Ptr && structField.Type.String() == "*bool" && !field.IsNil() {
		return true
	}
	if structField.Type.Kind() != reflect.Ptr && structField.Type.Kind() != reflect.Slice && !field.IsZero() {
		return true
	}
	return false
}

func setValue(structField reflect.StructField, field *reflect.Value, value string) error {
	switch structField.Type.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(value, 10, int(structField.Type.Size()*8))
		if err != nil {
			return err
		}
		field.SetInt(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := strconv.ParseUint(value, 10, int(structField.Type.Size()*8))
		if err != nil {
			return err
		}
		field.SetUint(v)
	case reflect.Float32, reflect.Float64:
		v, err := strconv.ParseFloat(value, int(structField.Type.Size()*8))
		if err != nil {
			return err
		}
		field.SetFloat(v)
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		field.SetBool(strings.ToLower(value) == "true")
	case reflect.Ptr:
		if structField.Type.String() == "*bool" {
			field.Set(reflect.ValueOf(structs.Ref(strings.ToLower(value) == "true")))
			return nil
		}
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return Parse(field.Interface())
	case reflect.Slice:
		if len(value) > 0 {
			values := strings.Split(value, ",")
			sl := reflect.MakeSlice(field.Type(), len(values), len(values))
			for i, val := range values {
				sl.Index(i).Set(reflect.ValueOf(val))
			}
			field.Set(sl)
		} else {
			for i := 0; i < field.Len(); i++ {
				if err := Parse(field.Index(i).Interface()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
