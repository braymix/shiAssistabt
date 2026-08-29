package com.shika.app

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.appcompat.app.AppCompatActivity
import androidx.core.app.ActivityCompat

// MainActivity starts the orchestrator service and shows the shikA dashboard
// (served by shikad on localhost) in a WebView. The device is a full worker
// node whether or not this screen is open — the service keeps running.
class MainActivity : AppCompatActivity() {

    private lateinit var web: WebView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        if (Build.VERSION.SDK_INT >= 33 &&
            checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            ActivityCompat.requestPermissions(this, arrayOf(Manifest.permission.POST_NOTIFICATIONS), 1)
        }

        val svc = Intent(this, ShikadService::class.java)
        if (Build.VERSION.SDK_INT >= 26) startForegroundService(svc) else startService(svc)

        web = WebView(this)
        web.settings.javaScriptEnabled = true
        web.settings.domStorageEnabled = true
        web.webViewClient = object : WebViewClient() {
            override fun onReceivedError(v: WebView, req: WebResourceRequest?, err: WebResourceError?) {
                // The dashboard may not be listening yet on first launch; retry.
                v.postDelayed({ v.loadUrl(DASH) }, 800)
            }
        }
        setContentView(web)
        // Give shikad a moment to bind its port before the first load.
        web.postDelayed({ web.loadUrl(DASH) }, 1200)
    }

    override fun onDestroy() {
        web.destroy()
        super.onDestroy()
    }

    companion object {
        const val DASH = "http://127.0.0.1:8977"
    }
}
