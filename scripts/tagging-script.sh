#!/bin/bash

GITHUB_REF="refs/heads/release/actiontest"
GITHUB_SHA="9a6f005d4e8c63b5fd3cca8db8e0c3ed90582081"
GITHUB_REF_TYPE="branch"

GITHUB_REF="refs/tags/1.5.3"
GITHUB_SHA="9a6f005d4e8c63b5fd3cca8db8e0c3ed90582081"
GITHUB_REF_TYPE="tag"

REGISTRY="ghcr.io"
IMAGE_NAME="donkeyx/tcp-wait"

set -eou pipefail

git_ref=""
hash=${GITHUB_SHA::6}

if [ "$GITHUB_REF_TYPE" == "tag" ]; then
  echo "processing tag"

  git_ref=$(echo ${GITHUB_REF#refs/tags/})

  # Set comma as delimiter

    #${REGISTRY}/${IMAGE_NAME}:
  new_ref=("$git_ref")
  
  IFS='.'
  for word in $git_ref; do
    new_ref+="${word}"
    echo "here\n"
  done
  
  echo ${new_ref[@]}
  exit
  docker_tag="${REGISTRY}/${IMAGE_NAME}:$git_ref"

fi

if [ "$GITHUB_REF_TYPE" == "branch" ]; then
  echo "processing branch"
  
  git_ref=$(echo ${GITHUB_REF#refs/heads/}| tr "/" "-" )

  if [ "$git_ref" == "master" ]; then
    echo "match latest"
    docker_tag=latest
  fi
  
#   docker_tag="${git_ref}_${hash}"
  docker_tag="${git_ref}"
fi

echo "git_ref: $git_ref"
echo "hash: $hash"
echo "dockertag: $docker_tag"

echo "::set-output name=git_ref::$git_ref"
echo "::set-output name=sha_short::$hash"
echo "::set-output name=docker_tag::$docker_tag"