#!/usr/bin/env python3
import sys
import os
import stat
import json
import argparse


def main():
    parser = argparse.ArgumentParser(
        description='Set or unset read-only attribute on files'
    )
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument('--set', action='store_true', help='Set read-only attribute')
    group.add_argument('--unset', action='store_true', help='Unset read-only attribute')

    args = parser.parse_args()

    # Read null-separated file paths from stdin
    input_data = sys.stdin.buffer.read()
    if not input_data:
        return

    # Split by null byte
    file_paths = input_data.split(b'\0')

    for file_path_bytes in file_paths:
        if not file_path_bytes:  # Skip empty entries
            continue

        file_path = file_path_bytes.decode('utf-8')

        try:
            if args.set:
                # Set read-only attribute (remove write permissions)
                current_mode = os.stat(file_path).st_mode
                new_mode = current_mode & ~stat.S_IWUSR & ~stat.S_IWGRP & ~stat.S_IWOTH
                os.chmod(file_path, new_mode)
                status = "set"
            else:  # args.unset
                # Unset read-only attribute (add write permission for owner)
                current_mode = os.stat(file_path).st_mode
                new_mode = current_mode | stat.S_IWUSR
                os.chmod(file_path, new_mode)
                status = "unset"

            # Output: path\0json
            output_json = json.dumps({"status": status})
            sys.stdout.buffer.write(file_path.encode('utf-8'))
            sys.stdout.buffer.write(b'\0')
            sys.stdout.buffer.write(output_json.encode('utf-8'))
            sys.stdout.buffer.write(b'\n')

        except Exception as e:
            # Output error response in same format
            output_json = json.dumps({"status": "error", "error": str(e)})
            sys.stdout.buffer.write(file_path.encode('utf-8'))
            sys.stdout.buffer.write(b'\0')
            sys.stdout.buffer.write(output_json.encode('utf-8'))
            sys.stdout.buffer.write(b'\n')


if __name__ == '__main__':
    main()
