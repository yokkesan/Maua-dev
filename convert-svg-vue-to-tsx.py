from pathlib import Path
import re

svg_dir = Path("src/frontend/src/components/svg")

replacements = {
    "stroke-width": "strokeWidth",
    "stroke-linecap": "strokeLinecap",
    "stroke-linejoin": "strokeLinejoin",
    "stroke-miterlimit": "strokeMiterlimit",
    "fill-rule": "fillRule",
    "clip-rule": "clipRule",
    "class=": "className=",
}

for vue_file in svg_dir.glob("*.vue"):
    content = vue_file.read_text(encoding="utf-8")

    match = re.search(r"<template>(.*?)</template>", content, re.S)

    if not match:
        print(f"skip: {vue_file.name}")
        continue

    svg = match.group(1).strip()

    for old, new in replacements.items():
        svg = svg.replace(old, new)

    component_name = vue_file.stem

    tsx = f"""export default function {component_name}() {{
  return (
{svg}
  )
}}
"""

    tsx_file = vue_file.with_suffix(".tsx")
    tsx_file.write_text(tsx, encoding="utf-8")

    print(f"{vue_file.name} -> {tsx_file.name}")