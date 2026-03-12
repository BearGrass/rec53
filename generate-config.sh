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

warmup:
  enabled: true
  timeout: 5s
  concurrency: 32
  tlds:
    - com
    - net
    - org
    - edu
    - gov
    - mil
    - int
    - info
    - biz
    - name
    - pro
    - asia
    - cat
    - coop
    - jobs
    - mobi
    - museum
    - tel
    - travel
    - aero
    - uk
    - cn
    - de
    - fr
    - jp
    - br
    - in
    - au
    - ca
    - ru
    - mx
    - es
    - it
    - nl
    - se
    - ch
    - no
    - be
    - at
    - dk
    - pl
    - gr
    - pt
    - tr
    - kr
    - tw
    - hk
    - sg
    - my
    - th
    - id
    - ph
    - vn
    - bd
    - pk
    - ng
    - za
    - eg
    - ke
    - nz
    - ie
    - il
    - ae
    - sa
    - ar
    - cl
    - co
    - ve
    - pe
    - ec
    - uy
    - eu
    - africa
    - americas
    - oceania
    - app
    - dev
    - io
    - cc
    - tv
    - co
    - xyz
    - online
    - cloud
    - tech
    - site
    - website
    - space
    - store
    - shop
    - blog
    - news
    - media
    - services
    - solutions
    - design
    - marketing
    - consulting
    - management
    - ventures
    - finance
    - insurance
    - bank
    - guru
    - expert
    - academy
    - education
    - school
    - university
    - college
    - training
    - courses
    - career
    - jobs
    - work
    - company
    - business
    - agency
    - studio
    - cafe
    - restaurant
    - bar
    - hotel
    - travel
    - tours
    - flights
    - booking
    - fitness
    - health
    - medical
    - hospital
    - clinic
    - dental
    - pharmacy
    - beauty
    - spa
    - salon
    - sports
    - games
    - gaming
    - esports
    - video
    - movie
    - cinema
    - music
    - artist
    - band
    - concert
    - theater
    - photography
    - photo
    - gallery
    - art
    - museum
    - fashion
    - luxury
    - jewelry
    - shoes
    - watch
    - wine
    - beer
    - coffee
    - food
    - pizza
    - burger
    - sushi
    - dance
    - religion
    - church
    - charity
    - ngo
    - foundation
    - club
    - community
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
