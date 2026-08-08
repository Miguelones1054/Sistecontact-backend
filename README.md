# SisteContact API (Backend en Go)

Backend que busca comercios por **tipo** dentro de una **zona** usando los datos
de Google Maps (Geocoding API + Places API New). Toda la lógica vive aquí; el
frontend solo pinta resultados.

## Flujo

1. El usuario escribe el tipo de comercio (p. ej. `barberías`).
2. El frontend pide zonas al backend: `GET /api/zones?q=chapinero`.
3. El usuario selecciona una zona de la lista.
4. El frontend busca: `GET /api/search?type=barberias&zone=<place_id o nombre>`.
5. El backend geocodifica la zona, calcula el radio del viewport y consulta
   Places API, devolviendo la lista de comercios.

## Endpoints

| Método | Ruta            | Query                          | Descripción                          |
|--------|-----------------|--------------------------------|--------------------------------------|
| GET    | `/api/health`   | —                              | Health check                         |
| GET    | `/api/zones`    | `q` (texto de zona)            | Zonas candidatas (geocoding)          |
| GET    | `/api/search`   | `type`, `zone`                 | Lista de comercios en la zona        |

## Configuración

1. Copia `.env.example` a `.env` y completa `GOOGLE_MAPS_API_KEY`.
2. Habilita en Google Cloud: **Geocoding API** y **Places API (New)**.

## Ejecución

```bash
cd api
export $(cat .env | xargs)   # o carga .env con tu gestor preferido
go mod tidy
go run ./cmd/server
```

Servidor en `http://localhost:8080`.

## Arquitectura

```
api/
├── cmd/server/main.go              # entrypoint + graceful shutdown
└── internal/
    ├── config/                     # config desde variables de entorno
    ├── model/                      # tipos de dominio (Zone, Business)
    ├── cache/                      # caché TTL genérica concurrente
    ├── geo/                        # utilidades geográficas (haversine)
    ├── googlemaps/                 # cliente Google (geocode + places)
    └── httpserver/                 # router, handlers, middleware
```

## Decisiones de rendimiento

- **Cero dependencias externas**: solo stdlib (Go 1.22+ ServeMux con rutas por método).
- **Connection pooling** en el `http.Transport` hacia Google.
- **Caché TTL en memoria** para zonas y resultados (reduce coste y latencia de Google).
- **Field masks** en Places API para traer solo los campos necesarios.
- **Graceful shutdown**, timeouts de lectura/escritura y `recover` de pánicos.
