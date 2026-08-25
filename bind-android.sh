#!/bin/bash
# Run from app/. Writes sibling Flutter AAR at ../mobile/android/app/libs/gtd.aar
# flutter run must not call this.
#
# ANDROID_HOME=~/Android/Sdk
# ANDROID_NDK_HOME=~/Android/Sdk/ndk/28.2.13676358
set -euo pipefail

cd "$(dirname "$0")"

gomobile bind \
  -target=android -androidapi 26 \
  -tags sqlite3_dotlk \
  -javapkg=dev.elsestudio.gtd \
  -o ../mobile/android/app/libs/gtd.aar \
  ./mobile
