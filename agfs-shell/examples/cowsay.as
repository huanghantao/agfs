#!/usr/bin/env agfs

# Cowsay Script
#
# Usage:
#   source cowsay.as
#   cowsay "Your message here"
#
# This script registers a cowsay function in the shell.
# The function displays a cow saying the provided message.

cowsay() {
    if [ -z "$1" ]; then
        local msg="Moo!"
    else
        local msg="$*"
    fi

    # Calculate message length using wc -c
    local len=$(echo "$msg" | wc -c)
    len=$((len - 1))

    # Build the speech bubble top border
    local border=""
    local i=0
    while [ $i -lt $((len + 2)) ]; do
        border="${border}_"
        i=$((i + 1))
    done

    # Build the speech bubble bottom border
    local border_bottom=""
    i=0
    while [ $i -lt $((len + 2)) ]; do
        border_bottom="${border_bottom}-"
        i=$((i + 1))
    done

    # Print the speech bubble
    echo " $border"
    echo "< $msg >"
    echo " $border_bottom"

    # Print the cow body - using single quotes to preserve literal characters
    echo '        \   ^__^'
    echo '         \  (oo)\_______'
    echo '            (__)\       )\/\'
    echo '                ||----w |'
    echo '                ||     ||'
}
