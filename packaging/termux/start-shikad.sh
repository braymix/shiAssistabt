#!/data/data/com.termux/files/usr/bin/sh
# Termux-services run script for shikA. Copy to
#   $PREFIX/var/service/shikad/run
# Keeps a wakelock so Android doesn't suspend the CPU while serving a model.
termux-wake-lock 2>/dev/null || true
exec "$HOME/shikad" -name "$(getprop ro.product.model 2>/dev/null || echo android)"
