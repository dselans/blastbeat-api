#!/usr/bin/env python
#
# Interactive script for executing make targets with a prefixed VERSION var
#

import sys
import json
import argparse
import subprocess
from datetime import datetime

# Define ANSI color codes as variables
GREEN = "\033[92m"
YELLOW = "\033[93m"
LIGHT_BLUE = "\033[94m"
LIGHT_CYAN = "\033[96m"
WHITE = "\033[97m"
RED = "\033[91m"
RESET = "\033[0m"

repo_name = ""
make_target = ""
fetch_images_command = """
aws ecr describe-images \
  --repository-name {} \
  --query 'imageDetails[?imageTags[0]!=`null`].[imageTags[0],imagePushedAt]' \
  --output json | \
  jq 'map(select(.[0] | test("makefile") | not)) | sort_by(.[1]) | reverse | .[:{}]'
"""

def display_help():
    print("Usage: ./deploy.py [-r <repo>] [-h]")
    print("  -r <repo>     The name of the ECR repository")
    print("  -t <target>   The make target to execute")
    print("  -l <limit>    Limit the number of images to fetch")
    print("  -h            Show this help message and exit")
    sys.exit(1)

def execute_command(command):
    try:
        result = subprocess.run(command, shell=True, capture_output=True, text=True)
        if result.returncode != 0:
            if result.stdout:
                output = result.stdout
            else:
                output = result.stderr

            print(color_text("ERROR: Command exited with '{}: {}'".format(result.returncode, output), RED))
            sys.exit(1)

        return result.stdout
    except Exception as e:
        print(color_text("ERROR: Exception when executing command: {}".format(e), RED))
        sys.exit(1)

def parse_json_output(json_output):
    if json_output == "":
        json_output = "[]"

    try:
        array_of_dicts = json.loads(json_output)
    except json.JSONDecodeError as e:
        print(color_text("ERROR: Unable to decode JSON: {}".format(e), RED))
        sys.exit(1)

    # Normalize date
    for item in array_of_dicts:
        try:
            item[1] = normalize_date(item[1])
        except Exception as e:
            # Leave as-is
            print(color_text("WARNING: Exception during date format: {}".format(e), YELLOW))
            item[1] = item[1]

    return array_of_dicts

def color_text(text, color):
    return f"{color}{text}{RESET}"

def display_choices(array_of_dicts):
    if len(array_of_dicts) == 0:
        print(color_text("WARNING: No images found for {}".format(repo_name), YELLOW))
        sys.exit(1)

    display_string = "Found {} images for {}".format(len(array_of_dicts), repo_name)
    separator = "=" * len(display_string)

    print(color_text(display_string, LIGHT_CYAN))
    print(color_text(separator, YELLOW))

    for i, item in enumerate(array_of_dicts, start=1):
        print(f"{color_text(str(i), GREEN)}: {color_text('Tag =', WHITE)} {color_text(item[0], YELLOW)}, {color_text('Date =', WHITE)} {color_text(item[1], LIGHT_BLUE)}")

    print(color_text(separator, YELLOW))

def normalize_date(date_str):
    dt = datetime.fromisoformat(date_str)
    return dt.strftime("%B %d %I:%M%p").lstrip('0')

def select_choice(images):
    try:
        choice = int(input(color_text("# to deploy: ", LIGHT_CYAN))) - 1

        if 0 <= choice < len(images):
            print(f"{color_text('Going to deploy:', GREEN)} {color_text('Tag =', WHITE)} {color_text(images[choice][0], YELLOW)}, {color_text('Date =', WHITE)} {color_text(images[choice][1], LIGHT_BLUE)}")
            input(color_text("Press [ENTER] to continue or CTRL-C to exit ...", LIGHT_CYAN))
            result = execute_command("VERSION={} && make {}".format(images[choice][0], make_target))
            print(result)
        else:
            print(color_text("ERROR: Invalid selection.", RED))
    except ValueError:
        print(color_text("ERROR: Invalid input. Please enter a number.", RED))

def handle_args():
    parser = argparse.ArgumentParser(description="Process some arguments.")

    parser.add_argument('-r', '--repo', type=str, help="Specify the repository name", required=True)
    parser.add_argument('-t', '--target', type=str, help="Specify the target", required=True)
    parser.add_argument('-f', '--filter', type=str, help="Filter image tags")
    parser.add_argument('-l', '--limit', type=int, help="Limit the number of images to fetch", default=20)

    return parser.parse_args()

def main():
    global repo_name
    global make_target

    args = handle_args()

    repo_name = args.repo
    make_target = args.target

    command = fetch_images_command.format(repo_name, args.limit)
    json_output = execute_command(command)

    # Replace the above JSON string with the actual output from your command.
    images = parse_json_output(json_output)

    # Prune list of images if filter is provided
    if args.filter is not None:
        images = [item for item in images if args.filter in item[0]]

    if len(images) == 0:
        print(color_text("WARNING: No images found for '{}' (filter: '{}')".format(repo_name, args.filter), YELLOW))
        sys.exit(1)

    display_choices(images)

    try:
        select_choice(images)
    except KeyboardInterrupt:
        print(color_text("\nCaught CTRL-C. Exiting ...", YELLOW))
        sys.exit(1)

if __name__ == "__main__":
    main()
