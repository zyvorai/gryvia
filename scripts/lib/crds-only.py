#!/usr/bin/env python3
"""Print only the CustomResourceDefinition documents from the given YAML files.

manifests/crds/*.yaml also hold example custom resources; those cannot be applied
until their CRD is established, so the deploy applies the CRDs alone.
"""
import re
import sys

for path in sys.argv[1:]:
    with open(path) as f:
        docs = re.split(r"(?m)^---\s*$", f.read())
    for doc in docs:
        if re.search(r"(?m)^kind:\s*CustomResourceDefinition\s*$", doc):
            print("---")
            print(doc.strip("\n"))
