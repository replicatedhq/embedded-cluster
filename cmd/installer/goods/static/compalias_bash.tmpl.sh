#!/bin/bash

_compalias()
{
    local alias command valarr fn
    alias="${COMP_WORDS[0]}"
    command="{{ .Command }}"

    read -ra valarr <<< "$command"
    COMP_WORDS=("${valarr[@]}" "${COMP_WORDS[@]:1}")
    COMP_LINE="${COMP_LINE//$alias/$command}"
    COMP_CWORD="$((${#COMP_WORDS[@]} - 1))"
    COMP_POINT="${#COMP_LINE}"

    # regex not perfect but good enough for 99%
    fn="$(complete -p "${COMP_WORDS[0]}" | grep -Po -- '-F\s+\K\w+')"

    # [-1] is generally faster than [$COMP_CWORD]
    "$fn" "${COMP_WORDS[0]}" "${COMP_WORDS[-1]}" "${COMP_WORDS[-2]}"
}

# nospace to prevent 2 spaces if default completion adds one
complete -o nospace -F _compalias {{ .Alias }}
