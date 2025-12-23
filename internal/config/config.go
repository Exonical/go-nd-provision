package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

func init() {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()
}

type Config struct {
	Server         ServerConfig
	Database       DatabaseConfig
	Valkey         ValkeyConfig
	NexusDashboard NexusDashboardConfig
	VCenter        VCenterConfig
}

type ServerConfig struct {
	Port string
	Mode string
}

type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	LogSQL          bool // Enable SQL query logging (verbose)
	MaxOpenConns    int  // Max open connections (0 = unlimited)
	MaxIdleConns    int  // Max idle connections
	ConnMaxLifetime int  // Connection max lifetime in minutes
}

type NexusDashboardConfig struct {
	BaseURL               string
	Username              string
	Password              string
	APIKey                string // Takes priority over username/password if set
	Insecure              bool
	ComputeFabricName     string
	ComputeVRFName        string
	ComputeNetworkName    string // Network name for security group selection
	ComputeAccessVLAN     string // Default access VLAN for compute interfaces (fallback if not in mapping)
	ComputeContractPrefix string // Optional prefix for job-specific contract names
	VMFabricName          string // VRF is per-tenant, not global
	SyncIntervalHours     int    // Interval for background sync of fabrics/switches/ports (0 = disabled)
}

type VCenterConfig struct {
	URL      string
	Username string
	Password string
	Insecure bool
}

type ValkeyConfig struct {
	Address  string
	Username string
	Password string
	DB       int
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Mode: getEnv("GIN_MODE", "debug"),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", "postgres"),
			DBName:          getEnv("DB_NAME", "nexus_dashboard"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			LogSQL:          getEnvBool("DB_LOG_SQL", false),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvInt("DB_CONN_MAX_LIFETIME", 30),
		},
		Valkey: ValkeyConfig{
			Address:  getEnv("VALKEY_ADDRESS", "localhost:6379"),
			Username: getEnv("VALKEY_USERNAME", "gond"),
			Password: getEnv("VALKEY_PASSWORD", "gond"),
			DB:       getEnvInt("VALKEY_DB", 0),
		},
		NexusDashboard: NexusDashboardConfig{
			BaseURL:               getEnv("ND_BASE_URL", "https://nexus-dashboard.example.com"),
			Username:              getEnv("ND_USERNAME", "admin"),
			Password:              getEnv("ND_PASSWORD", ""),
			APIKey:                getEnv("ND_API_KEY", ""),
			Insecure:              getEnvBool("ND_INSECURE", false),
			ComputeFabricName:     getEnv("ND_COMPUTE_FABRIC_NAME", ""),
			ComputeVRFName:        getEnv("ND_COMPUTE_VRF_NAME", ""),
			ComputeNetworkName:    getEnv("ND_COMPUTE_NETWORK_NAME", ""),
			ComputeAccessVLAN:     getEnv("ND_COMPUTE_ACCESS_VLAN", "2301"),
			ComputeContractPrefix: getEnv("ND_COMPUTE_CONTRACT_PREFIX", ""),
			VMFabricName:          getEnv("ND_VM_FABRIC_NAME", ""),
			SyncIntervalHours:     getEnvInt("ND_SYNC_INTERVAL_HOURS", 6),
		},
		VCenter: VCenterConfig{
			URL:      getEnv("VCENTER_URL", ""),
			Username: getEnv("VCENTER_USERNAME", ""),
			Password: getEnv("VCENTER_PASSWORD", ""),
			Insecure: getEnvBool("VCENTER_INSECURE", false),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
