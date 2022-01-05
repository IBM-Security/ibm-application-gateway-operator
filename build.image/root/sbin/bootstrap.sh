#!/bin/sh

##############################################################################
# Copyright contributors to the IBM Application Gateway Operator project
##############################################################################

# Set up the build area, symbolically linking files from our workspace.
mkdir -p /build

rsync -az /workspace/* /build

# Set the current working directory to the build area and then start a bash
# shell.
cd /build

/usr/bin/bash

