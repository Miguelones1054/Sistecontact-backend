# Guía de despliegue — SisteContact API
# Dominio: https://apisistecontact.nodefex.com
# App interna: 127.0.0.1:8091 (systemd)
# VPS: root@167.99.149.19

## Requisitos previos
1. DNS A: `apisistecontact.nodefex.com` → `167.99.149.19`
2. Archivo `.env` en `/root/Sistecontact-api/.env` (no va en git)
3. JSON de Firebase Admin en `/root/Sistecontact-api/` (no va en git)

## Setup inicial (una vez)

```bash
ssh root@167.99.149.19
```

### 1. Instalar Go (si no está)
```bash
curl -fsSL https://go.dev/dl/go1.25.0.linux-amd64.tar.gz -o /tmp/go.tgz
rm -rf /usr/local/go
tar -C /usr/local -xzf /tmp/go.tgz
echo 'export PATH=/usr/local/go/bin:$PATH' >> /root/.bashrc
export PATH=/usr/local/go/bin:$PATH
go version
```

### 2. Clonar repo
```bash
cd /root
git clone https://github.com/Miguelones1054/Sistecontact-backend.git Sistecontact-api
cd Sistecontact-api
```

### 3. Secrets
```bash
nano /root/Sistecontact-api/.env
# Subir el JSON de Firebase (desde tu Mac):
# scp ./sistecontact-firebase-adminsdk-....json root@167.99.149.19:/root/Sistecontact-api/
```

Contenido mínimo de `.env`:
```
GOOGLE_MAPS_API_KEY=...
PORT=8091
FIREBASE_CREDENTIALS_FILE=sistecontact-firebase-adminsdk-fbsvc-8adb9b6483.json
```

### 4. Build + systemd
```bash
export PATH=/usr/local/go/bin:$PATH
mkdir -p bin
go build -o bin/sistecontact-api ./cmd/server

cp deploy/sistecontact-api.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now sistecontact-api
systemctl status sistecontact-api
curl http://127.0.0.1:8091/api/health
```

### 5. Nginx
```bash
cp deploy/nginx-apisistecontact.nodefex.com.conf /etc/nginx/sites-available/apisistecontact.nodefex.com
ln -sf /etc/nginx/sites-available/apisistecontact.nodefex.com /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx
```

### 6. Certbot (HTTPS)
```bash
certbot --nginx -d apisistecontact.nodefex.com --non-interactive --agree-tos -m admin@nodefex.com --redirect
```

## Deploy diario (pull + rebuild + restart)

En el VPS:
```bash
bash /root/Sistecontact-api/deploy/deploy.sh
```

Desde tu Mac:
```bash
bash deploy/remote-deploy.sh
# o:
ssh root@167.99.149.19 'bash /root/Sistecontact-api/deploy/deploy.sh'
```

## Comandos útiles
```bash
journalctl -u sistecontact-api -f
systemctl restart sistecontact-api
systemctl status sistecontact-api
```
