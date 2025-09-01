#!/bin/bash

# Check if .env file exists
if [ ! -f ".env" ]; then
    echo "Error: .env file does not exist"
    echo "Please copy .env.example to .env first"
    exit 1
fi

# Get ngrok host from the API
echo "Getting ngrok host..."
NGROK_HOST=$(curl -s http://localhost:4040/api/tunnels/ | jq -r '.tunnels[0].public_url')

if [ "$NGROK_HOST" = "null" ] || [ -z "$NGROK_HOST" ]; then
    echo "Error: Could not get ngrok host. Make sure ngrok is running. Run make run/start/ngrok from a separate terminal to start ngrok."
    exit 1
fi

echo "Found ngrok host: $NGROK_HOST"

# Update the environment variable in .env file
echo "Updating NGROK_HOST in .env file..."

# Update the variable if it exists, otherwise add it
if grep -q "NGROK_HOST" .env; then
    # Update existing variable
    sed -i '' "s|NGROK_HOST=.*|NGROK_HOST=$NGROK_HOST|" .env
else
    # Add new variable at the end
    echo "NGROK_HOST=$NGROK_HOST" >> .env
fi

echo "Successfully updated .env file with ngrok host: $NGROK_HOST"
