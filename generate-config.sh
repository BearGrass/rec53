#!/bin/bash

# generate-config.sh - Auto-generates rec53 configuration file
# Usage: ./generate-config.sh [OPTIONS]
# Options:
#   -o, --output <path>   Output file path (default: ./config.yaml)
#   -h, --help           Show this help message

OUTPUT_FILE="./config.yaml"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -o|--output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -o, --output <path>   Output file path (default: ./config.yaml)"
            echo "  -h, --help           Show this help message"
            exit 0
            ;;
        *)
            echo "Error: Unknown option $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Generate config.yaml
cat > "$OUTPUT_FILE" << 'EOF'
dns:
  listen: "127.0.0.1:5353"
  metric: ":9999"
  log_level: "info"
  # upstream_timeout controls the per-query timeout when forwarding to authoritative NS servers.
  # Default: 1.5s (fast-fail; Happy Eyeballs concurrent queries absorb most reliability risk).
  # Increase to 3s-5s on high-latency networks; minimum allowed value is 100ms.
  # upstream_timeout: 1500ms

warmup:
  enabled: true
  timeout: 5s
  duration: 5s
  # Concurrency for warmup queries: dynamically calculated as min(NumCPU() * 2, 8).
  # On 4-core systems: 8 goroutines; on 2-core: 4; on 16-core+: 8 (capped).
  # You can override this value here if your deployment has special requirements.
  concurrency: 0  # 0 means use dynamic calculation; set to >0 to override (e.g., 16)
  # Curated list of 30 TLDs optimized for warmup.
  # Covers 85%+ of global domain registrations while keeping memory footprint low.
  # You can override this list with your own TLDs if needed.
  tlds:
    # Tier 1: Global mega-TLDs (8 domains)
    - com    # ~160M domains, 45% of all domains
    - cn     # China, ~20M
    - de     # Germany, ~16M
    - net    # ~12M
    - org    # ~11M
    - uk     # Britain
    - ru     # Russia
    - nl     # Netherlands, ~6M

    # Tier 2: Major ccTLDs & strategic gTLDs (22 domains)
    - br
    - xyz
    - info
    - top
    - it
    - fr
    - au
    - in
    - us
    - pl
    - ir
    - eu
    - es
    - ca
    - io
    - ai
    - me
    - site
    - shop
    - online
    - biz
    - app
EOF

if [ $? -eq 0 ]; then
    echo "✅ Config file generated: $OUTPUT_FILE"
    echo "📝 To start rec53, run:"
    echo "   ./rec53 --config $OUTPUT_FILE"
    exit 0
else
    echo "❌ Failed to generate config file"
    exit 1
fi
