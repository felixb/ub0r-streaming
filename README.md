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

The RTP config server manages a dynamic set of servers and receivers as they appear.
Radio streams are managed on the web frontend.

The following URIs are available as radio stream:

* `off`: turns off a server
* `alsa:${device}`: plays a alsa input stream
* `pulse:${device}`: plays a pule audio source
* `http...`: plays a web stream
* `test`: plays a sinus test signal

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
