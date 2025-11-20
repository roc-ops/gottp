#!/usr/bin/env python3
"""
Python TTP Runner for Comparison Tests

This script executes Python TTP with a given template and data,
then outputs the results as normalized JSON for comparison with GoTTP.
"""

import sys
import json
import argparse
import io
from contextlib import redirect_stdout
from pathlib import Path

# Add ttp-original to path if running from project root
sys.path.insert(0, str(Path(__file__).parent.parent.parent / "ttp-original"))

try:
    from ttp import ttp
except ImportError:
    print("ERROR: Python TTP not found. Please install it or ensure ttp-original is available.", file=sys.stderr)
    sys.exit(1)


def normalize_result(result):
    """
    Normalize TTP result for comparison.
    Handles various result structures and converts to JSON-serializable format.
    """
    if result is None:
        return None
    
    # If result is a list of lists (per_input method)
    if isinstance(result, list):
        return [normalize_result(item) for item in result]
    
    # If result is a dict
    if isinstance(result, dict):
        normalized = {}
        for key, value in sorted(result.items()):
            normalized[key] = normalize_result(value)
        return normalized
    
    # If result is a string, number, bool, or None, return as-is
    if isinstance(result, (str, int, float, bool)) or result is None:
        return result
    
    # For other types, convert to string representation
    return str(result)


def run_ttp(template_str, data_str=None, vars_dict=None, lookup_tables=None):
    """
    Run Python TTP with given template and data.
    
    Args:
        template_str: TTP template as string
        data_str: Input data as string (optional, can be in template)
        vars_dict: Template variables as dict (optional)
        lookup_tables: Lookup tables as dict (optional)
    
    Returns:
        Normalized result as JSON-serializable structure
    """
    try:
        # Python TTP defaults to results_method="per_input" if not specified
        # We don't need to wrap templates - Python TTP handles them as-is
        # Only convert results_method to results if present (for backwards compatibility)
        if "results_method=" in template_str:
            template_str = template_str.replace("results_method=", "results=")
        
        # Create parser
        parser = ttp(template=template_str)
        
        # Add data if provided
        if data_str:
            parser.add_input(data_str)
        
        # Add variables if provided
        if vars_dict:
            parser.add_vars(vars_dict)
        
        # Add lookup tables if provided
        if lookup_tables:
            for name, table in lookup_tables.items():
                parser.add_lookup(name, table)
        
        # Parse
        # If CSV format with terminal returner, Python TTP outputs CSV to stdout
        # We need to suppress this output for comparison tests
        # Capture stdout to prevent CSV output from interfering with JSON parsing
        f = io.StringIO()
        with redirect_stdout(f):
            parser.parse()
        
        # Get result
        result = parser.result()
        
        # Normalize and return
        return normalize_result(result)
    
    except Exception as e:
        return {
            "_error": True,
            "_error_message": str(e),
            "_error_type": type(e).__name__
        }


def main():
    parser = argparse.ArgumentParser(description="Run Python TTP and output JSON")
    parser.add_argument("--template", "-t", required=True, help="TTP template string or file path")
    parser.add_argument("--data", "-d", help="Input data string or file path")
    parser.add_argument("--vars", "-v", help="Variables as JSON string or file path")
    parser.add_argument("--lookups", "-l", help="Lookup tables as JSON string or file path")
    parser.add_argument("--template-file", action="store_true", help="Treat template as file path")
    parser.add_argument("--data-file", action="store_true", help="Treat data as file path")
    parser.add_argument("--vars-file", action="store_true", help="Treat vars as file path")
    parser.add_argument("--lookups-file", action="store_true", help="Treat lookups as file path")
    
    args = parser.parse_args()
    
    # Read template
    if args.template_file:
        with open(args.template, 'r') as f:
            template_str = f.read()
    else:
        template_str = args.template
    
    # Read data
    data_str = None
    if args.data:
        if args.data_file:
            with open(args.data, 'r') as f:
                data_str = f.read()
        else:
            data_str = args.data
    
    # Read vars
    vars_dict = None
    if args.vars:
        if args.vars_file:
            with open(args.vars, 'r') as f:
                vars_dict = json.load(f)
        else:
            vars_dict = json.loads(args.vars)
    
    # Read lookups
    lookup_tables = None
    if args.lookups:
        if args.lookups_file:
            with open(args.lookups, 'r') as f:
                lookup_tables = json.load(f)
        else:
            lookup_tables = json.loads(args.lookups)
    
    # Run TTP
    result = run_ttp(template_str, data_str, vars_dict, lookup_tables)
    
    # Output as JSON
    print(json.dumps(result, indent=2, sort_keys=True, ensure_ascii=False))
    
    # Exit with error code if there was an error
    if isinstance(result, dict) and result.get("_error"):
        sys.exit(1)


if __name__ == "__main__":
    main()

