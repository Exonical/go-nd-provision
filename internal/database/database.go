package database

import (
	"fmt"
	"net/url"
	"time"

	"github.com/banglin/go-nd/internal/config"
	"github.com/banglin/go-nd/internal/logger"
	"github.com/banglin/go-nd/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var DB *gorm.DB

// buildDSN constructs a properly escaped Postgres DSN URL.
// This handles special characters in password, user, dbname safely.
func buildDSN(cfg *config.DatabaseConfig) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   cfg.DBName,
	}
	q := u.Query()
	if cfg.SSLMode != "" {
		q.Set("sslmode", cfg.SSLMode)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func Initialize(cfg *config.DatabaseConfig) error {
	dsn := buildDSN(cfg)

	// Configure log level based on config (default: Warn to avoid noisy SQL logs)
	logLevel := gormlogger.Warn
	if cfg.LogSQL {
		logLevel = gormlogger.Info
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Minute)
	}

	// Ping to verify connection is actually working
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	logger.Info("Connected to PostgreSQL database")
	return nil
}

func Migrate() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	logger.Info("Running database migrations...")

	err := DB.AutoMigrate(
		&models.Fabric{},
		&models.Switch{},
		&models.SwitchPort{},
		&models.ComputeNode{},
		&models.ComputeNodeInterface{},
		&models.ComputeNodePortMapping{},
		&models.SecurityGroup{},
		&models.PortSelector{},
		&models.SecurityContract{},
		&models.ContractRule{},
		&models.SecurityAssociation{},
		&models.Job{},
		&models.JobComputeNode{},
		&models.ComputeNodeAllocation{},
		&models.Tenant{},
		&models.StorageTenant{},
		&models.JobStorageAccess{},
		&models.VM{},
	)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Info("Database migrations completed")
	return nil
}

func Close() error {
	if DB == nil {
		return nil // Already closed or never initialized
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
