package data

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Foundato/kelon/configs"
	"github.com/Foundato/kelon/pkg/data"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

var (
	//nolint:gochecknoglobals
	hostKey = "host"
	//nolint:gochecknoglobals
	portKey = "port"
	//nolint:gochecknoglobals
	dbKey = "database"
	//nolint:gochecknoglobals
	userKey = "user"
	//nolint:gochecknoglobals
	pwKey = "password"
)

func extractAndValidateDatastore(appConf *configs.AppConfig, alias string) (*configs.Datastore, error) {
	if appConf == nil {
		return nil, errors.New("AppConfig not configured! ")
	}
	if alias == "" {
		return nil, errors.New("Empty alias provided! ")
	}
	// Validate configuration
	conf, ok := appConf.Data.Datastores[alias]
	if !ok {
		return nil, errors.Errorf("No datastore with alias [%s] configured!", alias)
	}
	if strings.EqualFold(conf.Type, "") {
		return nil, errors.Errorf("Alias of datastore is empty! Must be one of %+v!", sql.Drivers())
	}
	if err := validateConnection(alias, conf.Connection); err != nil {
		return nil, err
	}
	return conf, nil
}

func pingUntilReachable(alias string, ping func() error) error {
	var pingFailure error
	for i := 0; i < 20; i++ {
		if pingFailure = ping(); pingFailure == nil {
			// Ping succeeded
			return nil
		}
		log.Infof("Waiting for [%s] to be reachable...", alias)
		<-time.After(3 * time.Second)
	}
	if pingFailure != nil {
		return errors.Wrap(pingFailure, "Unable to ping database")
	}
	return nil
}

func loadCallOperands(conf *configs.Datastore) (map[string]func(args ...string) string, error) {
	callOpsFile := fmt.Sprintf("./call-operands/%s.yml", strings.ToLower(conf.Type))
	handlers, err := LoadDatastoreCallOpsFile(callOpsFile)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to load call operands as handlers")
	}

	operands := map[string]func(args ...string) string{}
	for _, handler := range handlers {
		operands[handler.Handles()] = handler.Map
	}
	return operands, nil
}

func validateConnection(alias string, conn map[string]string) error {
	if _, ok := conn[hostKey]; !ok {
		return errors.Errorf("SqlDatastore: Field %s is missing in configured connection with alias %s!", hostKey, alias)
	}
	if _, ok := conn[portKey]; !ok {
		return errors.Errorf("SqlDatastore: Field %s is missing in configured connection with alias %s!", portKey, alias)
	}
	if _, ok := conn[dbKey]; !ok {
		return errors.Errorf("SqlDatastore: Field %s is missing in configured connection with alias %s!", dbKey, alias)
	}
	if _, ok := conn[userKey]; !ok {
		return errors.Errorf("SqlDatastore: Field %s is missing in configured connection with alias %s!", userKey, alias)
	}
	if _, ok := conn[pwKey]; !ok {
		return errors.Errorf("SqlDatastore: Field %s is missing in configured connection with alias %s!", pwKey, alias)
	}
	return nil
}

func getConnectionStringForPlatform(platform string, conn map[string]string) string {
	host, port, user, password, dbname, options := extractAndSortConnectionParameters(conn)

	switch platform {
	case data.TypePostgres:
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s%s", host, port, user, password, dbname, createConnOptionsString(options, " ", " "))
	case data.TypeMysql:
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s%s", user, password, host, port, dbname, createConnOptionsString(options, "&", "?"))
	case data.TypeMongo:
		return fmt.Sprintf("mongodb://%s:%s@%s:%s/%s%s", user, password, host, port, dbname, createConnOptionsString(options, "&", "?"))
	default:
		log.Panic(fmt.Sprintf("Platform [%s] is not a supported Datastore!", platform))
		return ""
	}
}

func getPreparePlaceholderForPlatform(platform string, argCounter int) string {
	switch platform {
	case data.TypePostgres:
		return fmt.Sprintf("$%d", argCounter)
	case data.TypeMysql:
		return "?"
	default:
		log.Panic(fmt.Sprintf("Platform [%s] is not a supported for prepared statements!", platform))
		return ""
	}
}

// Extract and sort all connection parameters by importance.
// Output: host, port, user, password, dbname, []options
// Each option has the format <key>=<value>
func extractAndSortConnectionParameters(conn map[string]string) (string, string, string, string, string, []string) {
	var host, port, user, password, dbname string
	var options []string

	for key, value := range conn {
		switch key {
		case hostKey:
			host = value
		case portKey:
			port = value
		case userKey:
			user = value
		case pwKey:
			password = value
		case dbKey:
			dbname = value
		default:
			options = append(options, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return host, port, user, password, dbname, options
}

func createConnOptionsString(options []string, delimiter string, prefix string) string {
	optionString := strings.Join(options, delimiter)
	if len(options) > 0 {
		optionString = prefix + optionString
	}
	return optionString
}
