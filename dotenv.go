package dotenv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cast"
)

var (
	DefaultConfigFile = ".env"
	DefaultSeparator  = "="
	// multiple config files cache: <file: <key: value>>
	mu           sync.RWMutex
	cachedConfig = make(map[string]map[string]interface{})
)

// DotEnv is a prioritized .env configuration registry.
// It maintains a set of configuration sources, fetches
// values to populate those, and provides them according
// to the source's priority.
// The priority of the sources is the following:
// 1. env. variables
// 2. config file
// 3. key/value store
// 4. defaults(when using structures)
//
// For example, if values from the following sources were loaded:
//
//	Defaults
//		USER=default
//		ENDPOINT=https://localhost
//
//	Config
//		USER=root
//		SECRET=secretFromConfig
//
//	Environment
//		SECRET=secretFromEnv
//
// The resulting config will have the following values:
//
//	SECRET=secretFromEnv
//	USER=root
//	ENDPOINT=https://localhost
//
// DotEnv is safe for concurrent Get___() and Set() operations by multiple goroutines.
type DotEnv struct {
	ConfigFile string

	// Separator is the symbol that separates the key-value pair.
	// Default is `=`
	Separator         string
	prefix            string
	allowEmptyEnvVars bool
}

// global DotEnv instance
var d *DotEnv

// Init returns an initialized DotEnv instance.
// Call this function as close as possible to the start of your program (ideally in main where your config file resides)
// If you call Init without any args it will default to loading .env in the current path
// You can otherwise tell it which file to load like
//
//	dotenv.Init("file.env")
func Init(file ...string) *DotEnv {
	var configFile string
	if len(file) > 0 {
		configFile = file[0]
	}

	// TODO: support multiple .env files in one instance

	if configFile == "" {
		configFile = DefaultConfigFile
	}

	dotenv := &DotEnv{
		ConfigFile: configFile,
		Separator:  DefaultSeparator,
	}

	return dotenv
}

// LoadConfig finds and read the config file.
// returns os.ErrNotExist if config file does not exist
func LoadConfig() error { return loadConfig() }

func loadConfig() error {
	if d == nil {
		d = Init()
	}
	return d.LoadConfig()
}

func (e *DotEnv) LoadConfig() (err error) {
	if !checkFileExists(e.ConfigFile) {
		return os.ErrNotExist
	}

	c, err := readAndParseConfig(e.ConfigFile, e.Separator)
	if err != nil {
		return err
	}
	cachedConfig[e.ConfigFile] = c
	return nil
}

// GetDotEnv returns the global DotEnv instance.
func GetDotEnv() *DotEnv {
	return d
}

// SetPrefix defines a prefix that ENVIRONMENT variables will use.
// E.g. if your prefix is "pro", the env registry will look for env
// variables that start with "PRO_".
func SetPrefix(prefix string) { d.SetPrefix(prefix) }

func (e *DotEnv) SetPrefix(prefix string) {
	e.prefix = strings.ToUpper(prefix) + "_"
}

// GetPrefix returns the prefix that ENVIRONMENT variables will use which is set with SetPrefix.
func GetPrefix() string { return d.GetPrefix() }

func (e *DotEnv) GetPrefix() string {
	return strings.TrimSuffix(e.prefix, "_")
}

func (e *DotEnv) addPrefix(key string) string {
	if e.prefix != "" {
		if !strings.HasPrefix(e.prefix, key) {
			key = e.prefix + key
		}
	}
	return key
}

// AllowEmptyEnv tells Dotenv to consider set, but empty environment variables
// as valid values instead of falling back to config value.
// This is set to true by default.
func AllowEmptyEnv(allowEmptyEnvVars bool) { d.AllowEmptyEnvVars(allowEmptyEnvVars) }

func (e *DotEnv) AllowEmptyEnvVars(allowEmptyEnvVars bool) {
	e.allowEmptyEnvVars = allowEmptyEnvVars
}

// SetConfigFile explicitly defines the path, name and extension of the config file.
// Dotenv will use this and not check .env from the current directory.
func SetConfigFile(configFile string) {
	if d != nil {
		d.SetConfigFile(configFile)
	}

	d = Init(configFile)
}

func (e *DotEnv) SetConfigFile(configFile string) {
	e.ConfigFile = configFile
}

// Get can retrieve any value given the key to use.
// Get is case-insensitive for a key.
// Dotenv will check in the following order:
// configOverride cache, env, key/value store, config file
//
// Get returns an interface. For a specific value use one of the Get___ methods e.g. GetBool(key) for a boolean value
func Get(key string) interface{} { return d.Get(key) }

func (e *DotEnv) Get(key string) interface{} {
	if key != "" {
		key = e.addPrefix(key)
		key = strings.ToUpper(key)

		if val, ok := os.LookupEnv(key); ok {
			if val == "" && !e.allowEmptyEnvVars {
				return val
			}
		}

		val, ok, err := GetFromFile(e.ConfigFile, key, e.Separator)
		if err == nil && ok {
			return val
		}

		// get from config file
		envFromFile, _, _ := getConfigValueWithKey(e.ConfigFile, key, e.Separator)

		return envFromFile
	}

	return ""
}

// GetString returns the value associated with the key as a string.
func GetString(key string) string { return d.GetString(key) }

func (e *DotEnv) GetString(key string) string {
	return cast.ToString(e.Get(key))
}

// GetBool returns the value associated with the key as a boolean.
func GetBool(key string) bool { return d.GetBool(key) }

func (e *DotEnv) GetBool(key string) bool {
	return cast.ToBool(e.Get(key))
}

// GetInt returns the value associated with the key as an integer.
func GetInt(key string) int { return d.GetInt(key) }

func (e *DotEnv) GetInt(key string) int {
	return cast.ToInt(e.Get(key))
}

// GetInt32 returns the value associated with the key as an integer.
func GetInt32(key string) int32 { return d.GetInt32(key) }

func (e *DotEnv) GetInt32(key string) int32 {
	return cast.ToInt32(e.Get(key))
}

// GetInt64 returns the value associated with the key as an integer.
func GetInt64(key string) int64 { return d.GetInt64(key) }

func (e *DotEnv) GetInt64(key string) int64 {
	return cast.ToInt64(e.Get(key))
}

// GetUint returns the value associated with the key as an unsigned integer.
func GetUint(key string) uint { return d.GetUint(key) }

func (e *DotEnv) GetUint(key string) uint {
	return cast.ToUint(e.Get(key))
}

// GetUint32 returns the value associated with the key as an unsigned integer.
func GetUint32(key string) uint32 { return d.GetUint32(key) }

func (e *DotEnv) GetUint32(key string) uint32 {
	return cast.ToUint32(e.Get(key))
}

// GetUint64 returns the value associated with the key as an unsigned integer.
func GetUint64(key string) uint64 { return d.GetUint64(key) }

func (e *DotEnv) GetUint64(key string) uint64 {
	return cast.ToUint64(e.Get(key))
}

// GetFloat64 returns the value associated with the key as a float64.
func GetFloat64(key string) float64 { return d.GetFloat64(key) }

func (e *DotEnv) GetFloat64(key string) float64 {
	return cast.ToFloat64(e.Get(key))
}

// GetTime returns the value associated with the key as time.
func GetTime(key string) time.Time { return d.GetTime(key) }

func (e *DotEnv) GetTime(key string) time.Time {
	return cast.ToTime(e.Get(key))
}

// GetDuration returns the value associated with the key as a duration.
func GetDuration(key string) time.Duration { return d.GetDuration(key) }

func (e *DotEnv) GetDuration(key string) time.Duration {
	return cast.ToDuration(e.Get(key))
}

// GetIntSlice returns the value associated with the key as a slice of int values.
func GetIntSlice(key string) []int { return d.GetIntSlice(key) }

func (e *DotEnv) GetIntSlice(key string) []int {
	return cast.ToIntSlice(e.Get(key))
}

// GetStringSlice returns the value associated with the key as a slice of strings.
func GetStringSlice(key string) []string { return d.GetStringSlice(key) }

func (e *DotEnv) GetStringSlice(key string) []string {
	return cast.ToStringSlice(e.Get(key))
}

// GetStringMap returns the value associated with the key as a map of interfaces.
func GetStringMap(key string) map[string]interface{} { return d.GetStringMap(key) }

func (e *DotEnv) GetStringMap(key string) map[string]interface{} {
	return cast.ToStringMap(e.Get(key))
}

// GetStringMapString returns the value associated with the key as a map of strings.
func GetStringMapString(key string) map[string]string { return d.GetStringMapString(key) }

func (e *DotEnv) GetStringMapString(key string) map[string]string {
	return cast.ToStringMapString(e.Get(key))
}

// GetStringMapStringSlice returns the value associated with the key as a map to a slice of strings.
func GetStringMapStringSlice(key string) map[string][]string { return d.GetStringMapStringSlice(key) }

func (e *DotEnv) GetStringMapStringSlice(key string) map[string][]string {
	return cast.ToStringMapStringSlice(e.Get(key))
}

// GetSizeInBytes returns the size of the value associated with the given key
// in bytes.
func GetSizeInBytes(key string) uint { return d.GetSizeInBytes(key) }

func (e *DotEnv) GetSizeInBytes(key string) uint {
	sizeStr := cast.ToString(e.Get(key))
	return parseSizeInBytes(sizeStr)
}

// IsSet checks to see if the key has been set in any of the env var, config cache or config file.
// IsSet is case-insensitive for a key.
func IsSet(key string) bool { return d.IsSet(key) }

func (e *DotEnv) IsSet(key string) bool {
	val := e.Get(key)
	return val != nil
}

// LookUp retrieves the value of the configuration named by the key.
// If the variable is present in the configuration file the value (which may be empty) is returned and the boolean is true.
// Otherwise the returned value will be empty and the boolean will be false.
func LookUp(key string) (interface{}, bool) { return d.LookUp(key) }

func (e *DotEnv) LookUp(key string) (interface{}, bool) {
	env, isSet, _ := GetFromFile(e.ConfigFile, key, e.Separator)
	return env, isSet
}

// Set sets or update env variable
// This will be used instead of following the normal precedence
// when getting the value
func Set(key, value string) { d.Set(key, value) }

func (e *DotEnv) Set(key string, value interface{}) {
	key = e.addPrefix(key)
	key = strings.ToUpper(key)

	mu.Lock()
	cachedConfig[e.ConfigFile][key] = value
	mu.Unlock()
}

// Save writes the current configuration to a file.
func Save() error { return d.Save() }

func (e *DotEnv) Save() error {
	cfgData := ""

	mu.RLock()
	defer mu.RUnlock()

	for key, value := range cachedConfig[e.ConfigFile] {
		cfgData += key + e.Separator + cast.ToString(value) + "\n"
	}
	return writeConfig(e.ConfigFile, cfgData)
}

// Write explicitly sets/update the configuration with the key-value provided
// and writes the current configuration to a file.
func Write(key string, value interface{}) error { return d.Write(key, value) }

func (e *DotEnv) Write(key string, value interface{}) error {
	e.Set(key, value)
	return e.Save()
}

// InvalidateEnvCacheForFile invalidates the cached content of a file
func InvalidateEnvCacheForFile(filePath string) {
	mu.Lock()
	delete(cachedConfig, filePath)
	mu.Unlock()
}

// GetFromFile retrieves the value of the config variable named by the key from the config file
// If the variable is present in the environment the value (which may be empty) is returned and the boolean is true.
// Otherwise the returned value will be empty and the boolean will be false.
func GetFromFile(filePath, key, separator string) (interface{}, bool, error) {
	if !checkFileExists(filePath) {
		return "", false, os.ErrNotExist
	}

	mu.Lock()
	defer mu.Unlock()

	configCache, okConfig := cachedConfig[filePath]
	if !okConfig {
		c, err := readAndParseConfig(filePath, separator)
		if err != nil {
			return nil, false, err
		}
		configCache = c
		if cachedConfig == nil {
			cachedConfig = make(map[string]map[string]interface{})
		}
		cachedConfig[filePath] = configCache
	}

	if cachedEnv, okEnv := configCache[key]; okEnv {
		return cachedEnv, true, nil
	}

	return "", false, nil
}

func getConfigValueWithKey(configFile, key, separator string) (env interface{}, exists bool, err error) {
	// first get os env var
	env = os.Getenv(key)

	if env == "" {
		// Find config variable in config file
		env, exists, err = GetFromFile(configFile, key, separator)
	}
	return
}

func writeConfig(cfgFile, data string) error {
	defer InvalidateEnvCacheForFile(cfgFile)

	_ = os.MkdirAll(filepath.Join(cfgFile, ".."), 0755)
	if err := WriteFile(cfgFile, []byte(data), 0666); err != nil {
		return fmt.Errorf("failed to write to config file: %q", err)
	}

	return nil
}

// checkFileExists returns true if a file exists and is not a directory.
func checkFileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
