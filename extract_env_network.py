#!/usr/bin/env python3

import re
import sys
from urllib.parse import urlparse

def parse_env(path):
    data = {}

    with open(path) as f:
        for line in f:
            line=line.strip()

            if not line or line.startswith("#"):
                continue

            if "=" not in line:
                continue

            k,v=line.split("=",1)

            k=k.strip()
            v=v.strip().strip('"').strip("'")

            data[k]=v

    return data


def extract_from_url(value):

    try:
        p=urlparse(value)

        if not p.scheme or not p.hostname:
            return None

        port=p.port

        if not port:
            if p.scheme=="http":
                port=80
            elif p.scheme=="https":
                port=443
            elif p.scheme=="mysql":
                port=3306
            elif p.scheme=="redis":
                port=6379

        if not port:
            return None

        return f"{p.scheme}://{p.hostname}:{port}"

    except:
        return None


def main(envfile):

    env=parse_env(envfile)

    results=set()

    host_map={}
    port_map={}

    for k,v in env.items():

        # URL
        if "://" in v:
            r=extract_from_url(v)
            if r:
                results.add(r)

        # host变量
        if "HOST" in k.upper():
            base=k.upper().replace("HOST","")
            host_map[base]=v

        # port变量
        if "PORT" in k.upper():
            base=k.upper().replace("PORT","")
            port_map[base]=v

    # 组合 host + port
    for base in host_map:

        if base in port_map:

            host=host_map[base]
            port=port_map[base]

            results.add(f"tcp://{host}:{port}")

    for r in sorted(results):
        print(r)


if __name__ == "__main__":

    if len(sys.argv)<2:
        print("usage: python extract_endpoints.py .env")
        sys.exit(1)

    main(sys.argv[1])