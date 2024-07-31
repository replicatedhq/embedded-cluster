#!/bin/bash

_compalias()
{
    local name val valarr fn
    name="${COMP_WORDS[0]}"
    val="${BASH_ALIASES[$name]}"

    [ -z "$val" ] && return 1
    read -ra valarr <<< "$val"
    COMP_WORDS=("${valarr[@]}" "${COMP_WORDS[@]:1}")
    COMP_LINE="${COMP_LINE//$name/$val}"
    COMP_CWORD="$((${#COMP_WORDS[@]} - 1))"
    COMP_POINT="${#COMP_LINE}"

    # regex not perfect but good enough for 99%
    fn="$(complete -p "${COMP_WORDS[0]}" | grep -Po -- '-F\s+\K\w+')"

    # [-1] is generally faster than [$COMP_CWORD]
    "$fn" "${COMP_WORDS[0]}" "${COMP_WORDS[-1]}" "${COMP_WORDS[-2]}"
}

compalias()
{
    builtin alias "$@"
    # nospace to prevent 2 spaces if default completion adds one
    complete -o nospace -F _compalias "${@%%=*}"
}

# it also supports multiple aliases at once now!
compalias {{ .Alias }}='{{ .Command }}'
