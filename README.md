# ub0r audio streaming

This is a tool set of small programs helping to stream audio over a local network.
It's kind of multi room aware and supports possibly synchronous playback of a single stream on different players.

There are three components necessary to stream audio.

## RTP config

A small config server managing which stream should run on which device.
It provides the frontend to the user.

You'll need one config server.

## RTP sender

The sender encodes a web radio stream or line in into a opus+ogg stream and provides this compressed stream as a TCP server to the local network.
It's basically a thin layer around gstreamer.

You can run a stand alone server on any device or let the RTP config server spawn them in the background.

## RTP receiver

The receiver is an even thinner layer around gstreamer.
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

# Screenshots

The RTP config server has a ub0r web UI.
It's responsive designed and uses bleeding edge web techniques:

![Receivers][screen_receiver]
![Radios][screen_radios]

# Dependencies for running

You need gstreamer 1.0 including some plugins to run ub0r streaming.

On debian/ubuntu:

    apt-get install gstreamer1.0-alsa gstreamer1.0-pulseaudio gstreamer1.0-plugins-good gstreamer1.0-plugins-bad

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

[screen_server]: https://raw.githubusercontent.com/felixb/ub0r-streaming/master/assets/screen_server_small.png "Screenshot: Servers"
[screen_receiver]: https://raw.githubusercontent.com/felixb/ub0r-streaming/master/assets/screen_receiver_small.png "Screenshot: Receivers"
[screen_radios]: https://raw.githubusercontent.com/felixb/ub0r-streaming/master/assets/screen_radios_small.png "Screenshot: Radios"
