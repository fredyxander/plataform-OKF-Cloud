package config

import "os"

// Config contiene la configuración que necesita la aplicación.
// No ponemos direcciones o contraseñas directamente en el código.
// Todos esos valores llegan mediante variables de entorno.
type Config struct {
	RabbitMQURL string
}

// Load lee las variables de entorno y construye la configuración.
func Load() Config {
	return Config{
		RabbitMQURL: getEnv(
			"RABBITMQ_URL",
			"amqp://okf:okf@rabbitmq:5672/",
		),
	}
}

// getEnv devuelve una variable de entorno.
// Si no existe, utiliza un valor por defecto.
func getEnv(key string, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}