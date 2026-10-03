#!/bin/bash

# Script para compilar o plugin DuckDB Datasource para ARM64
# Este script é necessário porque o plugin não tem binários pré-compilados para ARM64

set -e

echo "🔨 Compilando plugin DuckDB Datasource para ARM64..."

# Verificar se Go está instalado
if ! command -v go &> /dev/null; then
    echo "❌ Go não está instalado. Instale Go 1.24.1 ou superior."
    exit 1
fi

# Verificar versão do Go
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "✅ Go versão: $GO_VERSION"

# Verificar se mage está instalado
if ! command -v mage &> /dev/null; then
    echo "📦 Instalando Mage..."
    go install github.com/magefile/mage@latest
fi

# Instalar dependências do frontend
echo "📦 Instalando dependências do frontend..."
npm install

# Build do frontend
echo "🎨 Compilando frontend..."
npm run build

# Build do backend com CGO habilitado para ARM64
echo "🔧 Compilando backend para ARM64..."
export CGO_ENABLED=1
export GOOS=linux
export GOARCH=arm64

mage -v build:linux

echo "✅ Build concluído! Plugin compilado em ./dist/"
echo ""
echo "Para usar com Docker:"
echo "  docker-compose up"
echo ""
echo "Para instalar manualmente:"
echo "  sudo cp -r ./dist /var/lib/grafana/plugins/motherduck-duckdb-datasource"
echo "  sudo systemctl restart grafana-server"
