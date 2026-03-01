package config

import (
	"os"
	"reflect"

	viperlib "github.com/spf13/viper"
)

var viper *viperlib.Viper

type ConfigFunc func() map[string]interface{}

var ConfigFuncs map[string]ConfigFunc

func init() {
	viper = viperlib.New()
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	ConfigFuncs = make(map[string]ConfigFunc)
}

func InitConfig(env string) {
	loadEnv(env)
	loadConfig()
}

func loadConfig() {
	for name, fn := range ConfigFuncs {
		viper.Set(name, fn())
	}
}

func loadEnv(envSuffix string) {
	envPath := ".env"
	if envSuffix != "" {
		candidate := ".env." + envSuffix
		if _, err := os.Stat(candidate); err == nil {
			envPath = candidate
		}
	}

	viper.SetConfigName(envPath)
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}
	viper.WatchConfig()
}

func Env(envName string, defaultValue ...interface{}) interface{} {
	if len(defaultValue) > 0 {
		return internalGet(envName, defaultValue[0])
	}
	return internalGet(envName)
}

func Add(name string, configFn ConfigFunc) {
	ConfigFuncs[name] = configFn
}

func Get[T any](path string, defaultValue ...interface{}) T {
	value := internalGet(path, defaultValue...)
	var fallback T
	if value == nil {
		return fallback
	}

	if v, ok := value.(T); ok {
		return v
	}

	targetType := reflect.TypeOf(fallback)
	val := reflect.ValueOf(value)
	if targetType != nil && val.Type().ConvertibleTo(targetType) {
		return val.Convert(targetType).Interface().(T)
	}

	return fallback
}

func internalGet(path string, defaultValue ...interface{}) interface{} {
	value := viper.Get(path)
	if !viper.IsSet(path) || value == nil || value == "" {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return nil
	}
	return value
}
