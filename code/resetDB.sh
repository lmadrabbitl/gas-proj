#!/bin/bash
set -euo pipefail

echo "Resetting migrations"
make migrate-reset

echo "Database reset complete. Project seeding was removed."

