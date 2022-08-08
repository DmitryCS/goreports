package config

import (
	"os"
	"strconv"
)

func GetStringEnv(envName string, defaultValue string) string {
	var value = os.Getenv(envName)
	if len(value) > 0 {
		return value
	}
	return defaultValue
}

func GetIntEnv(envName string, defaultValue int) int {
	var value = os.Getenv(envName)
	if len(value) > 0 {
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return defaultValue
		}
		return intValue
	}
	return defaultValue
}

func GetBoolEnv(envName string, defaultValue bool) bool {
	var value = os.Getenv(envName)
	if len(value) > 0 {
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return defaultValue
		}
		return intValue == 1
	}
	return defaultValue
}

func GetFloat64Env(envName string, defaultValue float64) float64 {
	var value = os.Getenv(envName)
	if len(value) > 0 {
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return defaultValue
		}
		return floatValue
	}
	return defaultValue
}
