#!/usr/bin/env bash
#
# This is a shared shell script that contains common helper functions used by
# other shell scripts and the main 'Makefile'.
#

BOLD='\033[1m'              # Bold text
RESET='\033[0m'             # Reset colors
DEBUG_COLOR='\033[30;46m'   # Black text on Cyan background
INFO_COLOR='\033[30;42m'    # Black text on Green background
WARNING_COLOR='\033[30;43m' # Black text on Yellow background
FATAL_COLOR='\033[37;41m'   # White text on Red background
DEPLOYMENT_MSG=$(cat << 'EOF'
{
	"blocks": [
		{
			"type": "context",
			"elements": [
				{
					"type": "mrkdwn",
					"text": "Service: *<https://github.com/__ORG__/__SERVICE__|__SERVICE__>*"
				},
				{
					"type": "mrkdwn",
					"text": "Version: *<https://github.com/__ORG__/__SERVICE__/commit/__VERSION__|__VERSION__>*"
				},
				{
					"type": "mrkdwn",
					"text": "Makefile Target: *__TARGET__*"
				},
				{
					"type": "mrkdwn",
					"text": "User: *__USER__*"
				}
			]
		},
		{
			"type": "section",
			"text": {
				"type": "mrkdwn",
				"text": "__MESSAGE__"
			}
		}
	]
}
EOF
)

######################## Display funcs ##############################

# Function to pad the message to a minimum length of 120 characters
pad_message() {
    local message="$1"
    local padding=$(printf '%0.1s' " "{1..120})
    echo "${message}${padding}"
}

# Function to print a message with a colored background
print_colored_line() {
    local color="$1"
    local label="$2"
    local message="$3"
    local padded_message=$(pad_message "${label} ${message}")
    printf "%b%s%b\n" "${color}${BOLD}" "${padded_message:0:120}" "${RESET}"
}

debug() {
    print_colored_line "${DEBUG_COLOR}" "‣ [DEBUG]" "$*"
}

info() {
    print_colored_line "${INFO_COLOR}" "‣ [INFO]" "$*"
}

warning() {
    print_colored_line "${WARNING_COLOR}" "‣ [WARNING]" "$*"
}

fatal() {
    print_colored_line "${FATAL_COLOR}" "‣ [FATAL]" "$*"
    exit 1
}

######################## Helper funcs #######################

prereq() {
    # Do all the required tools exist?
    REQUIRED_TOOLS=("git" "curl" "docker" "jq" "kubectl" "aws" "doppler")

    # If this script is running in a Github Workflow, we do not need some of the tools
    if [[ -n "${GITHUB_ACTIONS}" ]]; then
        REQUIRED_TOOLS=("git" "curl" "docker" "jq")
    fi

    for tool in "${REQUIRED_TOOLS[@]}"; do
        if ! command -v "${tool}" > /dev/null; then
            fatal "Required tool '${tool}' not found"
        fi
    done
}

check_vars() {
    for var in "$@"; do
        if [ -z "${!var}" ]; then
            fatal "'$var' is not set"
        fi
    done
}

####################### Slack notification #######################

notify() {
    # Verify that vars required for notification are set
    vars=("SERVICE" "ORG" "VERSION" "TARGET")

    check_vars "${vars[@]}"

    # SLACK_WEBHOOK_URL must exist to send a message - if it's not set, try to
    # fetch it via doppler.
    if [ -z "${SLACK_WEBHOOK_URL}" ]; then
        SLACK_WEBHOOK_URL=$(doppler secrets get DN_SLACK_WEBHOOK_URL -p shared -c prd --plain)

        if [ $? -ne 0 ]; then
            warning "Unable to fetch SLACK_WEBHOOK_URL from doppler"
            return 1
        fi
    fi

    FULL_DEPLOYMENT_MSG=$(echo "${DEPLOYMENT_MSG}" | \
        sed "s|__ORG__|${ORG}|g" | \
        sed "s|__SERVICE__|${SERVICE}|g" | \
        sed "s|__VERSION__|${VERSION}|g" | \
        sed "s|__TARGET__|${TARGET}|g" | \
        sed "s|__USER__|${USER}|g" | \
        sed "s|__MESSAGE__|$1|g" |
        jq -cRr .)

    curl -X POST -H 'Content-type: application/json' --data "${FULL_DEPLOYMENT_MSG}" $SLACK_WEBHOOK_URL
}

#################### Function execution logic ###################

# Perform check regardless of whether the script is sourced or not
prereq

# This script is either sourced or called directly from the Makefile. If it is
# called with an arg, execute a function with the same name as the $1 arg.
# IMPORTANT: This check MUST be last in the script, otherwise it will not be
# able to properly lookup func names.

sourced=0
if [ "${BASH_SOURCE[0]}" != "${0}" ]; then
    sourced=1
fi

# If the script is sourced, skip the function execution
if [ "$sourced" -eq 1 ]; then
    return 0
fi

# If an argument is provided, call the function with the same name and pass any additional arguments
if [ $# -gt 0 ]; then
    function_name="$1"
    shift
    if declare -F "$function_name" > /dev/null; then
        "$function_name" "$@"
    else
        fatal "Function '$function_name' not found"
    fi
fi
