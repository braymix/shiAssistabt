package com.shika.app

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.net.wifi.WifiManager
import android.os.Build
import android.os.IBinder
import android.os.PowerManager
import android.util.Log
import java.io.File

// ShikadService runs the shikad orchestrator binary for the life of the app.
// The binary ships as libshikad.so (so it lands in the read-only, executable
// nativeLibraryDir) and is launched as a normal child process. A wakelock keeps
// the CPU alive while serving, and a multicast lock lets LAN auto-discovery
// receive beacons.
class ShikadService : Service() {

    private var proc: Process? = null
    private var wake: PowerManager.WakeLock? = null
    private var multicast: WifiManager.MulticastLock? = null

    @Volatile
    private var running = false

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        startForeground(NOTIF_ID, buildNotification())
        acquireLocks()
        startShikad()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int = START_STICKY

    private fun startShikad() {
        if (running) return
        val bin = File(applicationInfo.nativeLibraryDir, "libshikad.so")
        if (!bin.canExecute()) {
            Log.e(TAG, "shikad binary not found or not executable at ${bin.absolutePath}")
            stopSelf()
            return
        }
        val name = Build.MODEL ?: "android"
        Thread {
            running = true
            try {
                val pb = ProcessBuilder(bin.absolutePath, "-name", name)
                pb.redirectErrorStream(true)
                pb.directory(filesDir) // writable cwd for any state
                proc = pb.start()
                proc?.inputStream?.bufferedReader()?.forEachLine { Log.i(TAG, it) }
                proc?.waitFor()
            } catch (e: Exception) {
                Log.e(TAG, "shikad exited abnormally", e)
            } finally {
                running = false
            }
        }.start()
    }

    private fun acquireLocks() {
        val pm = getSystemService(Context.POWER_SERVICE) as PowerManager
        wake = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "shikA:cpu").apply {
            setReferenceCounted(false)
            acquire()
        }
        val wifi = applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
        multicast = wifi.createMulticastLock("shikA:discovery").apply {
            setReferenceCounted(false)
            acquire()
        }
    }

    private fun buildNotification(): Notification {
        if (Build.VERSION.SDK_INT >= 26) {
            val ch = NotificationChannel(CHANNEL, "shikA", NotificationManager.IMPORTANCE_LOW)
            (getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager)
                .createNotificationChannel(ch)
        }
        val open = PendingIntent.getActivity(
            this, 0, Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE
        )
        val builder = if (Build.VERSION.SDK_INT >= 26) {
            Notification.Builder(this, CHANNEL)
        } else {
            @Suppress("DEPRECATION") Notification.Builder(this)
        }
        return builder
            .setContentTitle("shikA is running")
            .setContentText("This device is part of your AI mesh")
            .setSmallIcon(android.R.drawable.stat_sys_upload_done)
            .setContentIntent(open)
            .setOngoing(true)
            .build()
    }

    override fun onDestroy() {
        try {
            proc?.destroy()
        } catch (_: Exception) {
        }
        wake?.let { if (it.isHeld) it.release() }
        multicast?.let { if (it.isHeld) it.release() }
        super.onDestroy()
    }

    companion object {
        private const val TAG = "shikad"
        private const val CHANNEL = "shika"
        private const val NOTIF_ID = 1
    }
}
