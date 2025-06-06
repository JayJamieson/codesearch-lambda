#!/usr/bin/env bash

set -euo pipefail

# Default AWS Lambda base URL
BASE_URL="https://jiu7kd3heflrxqh6hp36t5cl2i0pvpke.lambda-url.ap-southeast-2.on.aws"

print_usage() {
  cat <<EOF
Usage:
  cs -index <repo_url>
  cs -search -repo <repo_url> -query "<search query>"

Options:
  -index       Index the given GitHub repo (POST to /cindex)
  -search      Search indexed repo (GET from /csearch)
  -a           Arguments (required for -search)
  -q           Search query string (required for -search)
  -h           Show this help message
EOF
}

# No args? show usage
if [ $# -eq 0 ]; then
  print_usage
  exit 1
fi

# Parse top‐level command
case "$1" in
  -index)
    shift
    if [ $# -ne 1 ]; then
      echo "Error: -index requires exactly one <repo_url> argument."
      print_usage
      exit 1
    fi
    REPO_URL="$1"
    echo "Indexing repo: $REPO_URL"
    curl -X POST \
         -H "Content-Type: application/json" \
         -d "{\"repo\":\"$REPO_URL\"}" \
         "$BASE_URL/cindex"
    ;;

  -search)
    shift
    # Initialize variables
    ARGS=""
    QUERY=""
    # Parse flags for search
    while getopts ":q:a:h" opt; do
      # echo $OPTARG
      case ${opt} in
        a) ARGS="$OPTARG" ;;
        q) QUERY="$OPTARG" ;;
        h) print_usage; exit 0 ;;
        \?) echo "Invalid option: -$OPTARG" >&2; exit 1 ;;
        :)  echo "Option -$OPTARG requires an argument." >&2; exit 1 ;;
      esac
    done
    # Validate
    if [ -z "$ARGS" ] || [ -z "$QUERY" ]; then
      echo "Error: -search requires both -a and -q."
      print_usage
      exit 1
    fi
    echo "Searching for query: \"$QUERY\" with args \"$ARGS\""

    curl -G \
         --data-urlencode "args=$ARGS" \
         --data-urlencode "q=$QUERY" \
         "$BASE_URL/csearch"
    ;;

  -h|--help)
    print_usage
    ;;

  *)
    echo "Unknown command: $1" >&2
    print_usage
    exit 1
    ;;
esac
