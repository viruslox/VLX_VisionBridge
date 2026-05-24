#!/bin/bash
xvfb-run --server-num=99 --server-args="-screen 0 1920x1080x24" chromium --version
