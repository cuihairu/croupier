#!/usr/bin/env python3
import os
import re

def camel_to_snake(name):
    """Convert camelCase to snake_case"""
    name = name.replace('handler', '').replace('logic', '')
    name = name.replace('servicecontext', 'service_context')

    # Simple heuristic: split before common prefixes and suffixes
    prefixes = ['auth', 'role', 'user', 'game', 'admin', 'agent', 'job', 'pack', 'meta', 'ops', 'support']
    suffixes = ['detail', 'list', 'create', 'update', 'delete', 'add', 'get', 'check', 'verify', 'login', 'logout', 'me']

    result = name
    for prefix in prefixes:
        if result.startswith(prefix):
            result = prefix + '_' + result[len(prefix):]
            break

    for suffix in suffixes:
        if result.endswith(suffix):
            result = result[:-len(suffix)] + '_' + suffix
            break

    return result.replace('__', '_') + ('.go')

def rename_files():
    """Rename all handler and logic files to snake_case"""
    count = 0

    for root, dirs, files in os.walk('services/server/internal'):
        for file in files:
            if file.endswith('handler.go') or file.endswith('logic.go'):
                old_path = os.path.join(root, file)

                # Convert filename
                name_without_ext = file[:-3]  # Remove .go
                new_name = camel_to_snake(name_without_ext) + '.go'
                new_path = os.path.join(root, new_name)

                if old_path != new_path:
                    print(f"Renaming: {old_path} -> {new_path}")
                    os.rename(old_path, new_path)
                    count += 1

    print(f"Renamed {count} files to snake_case style!")

if __name__ == "__main__":
    rename_files()