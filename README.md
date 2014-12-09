# ub0r audio streaming

This is a tool set of small programs helping to stream audio over a local network.
It's kind of multi room aware and supports possibly synchronous playback of a single stream on different players.

There are three components necessary to stream audio.

## RTP config

A small config server managing which stream should run on which device.
It provides the frontend to the user.

You'll need one config server.

## RTP sender

The server encodes a web radio stream or line in into a opus+ogg stream and provides this compressed stream as a TCP server to the local network.
It's basically a thin layer around gstreamer.

You'll need at least one sender.

## RTP receiver

The sender is an even thinner layer around gstreamer.
It connects to one of the sender components to decode and play the compressed audio stream.

You'll need at least one receiver.

# Configuration

The RTP config server needs a list of streams, servers and receivers to manage the system.
The configuration is made in yaml format:

    # list of radio streams a server can play
    radios:
      - name: off
        uri: off
      - name: Line in
        uri: alsa:hw:1
      - name: Bass Drive
        uri: http://amsterdam2.shouthost.com.streams.bassdrive.com:8000
      - name: Audio Test
        uri: test
    # a list of servers to manage
    servers:
      - name: Living room
        host: rsp0
        port: 48100
    # a list of static unmanaged servers
    # e.g. for streaming audio from your pc into the network
    staticservers:
      - name: Notebook
        host: t420
        port: 48100
    # a list of receivers who are allowed to connect to any server
    receivers:
      - name: Living room
        host: rsp0
      - name: Kitchen
        host: rsp1

# Build

The simplest way is using make.

    make all

# Dependencies for building

You need gstreamer 1.0 dev files to build the dependencies.

On debian/ubuntu:

    apt-get install libgstreamer1.0-dev

# Contributing

Just fork and send PR.
Please try to stick to current code style.

# License

Copyright 2014 Felix Bechstein

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
