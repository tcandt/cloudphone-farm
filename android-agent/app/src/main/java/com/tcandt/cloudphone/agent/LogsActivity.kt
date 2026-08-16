package com.tcandt.cloudphone.agent

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.os.Bundle
import android.text.Editable
import android.text.TextWatcher
import android.widget.Button
import android.widget.EditText
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import com.tcandt.cloudphone.agent.logging.AgentLogStore

class LogsActivity : AppCompatActivity() {

    private lateinit var logStore: AgentLogStore
    private lateinit var tvLogsContent: TextView
    private lateinit var etLogSearch: EditText

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_logs)

        logStore = AgentLogStore.getInstance(this)
        tvLogsContent = findViewById(R.id.tvLogsContent)
        etLogSearch = findViewById(R.id.etLogSearch)

        val btnCopy = findViewById<Button>(R.id.btnCopyLogs)
        val btnClear = findViewById<Button>(R.id.btnClearLogs)

        renderLogs("")

        etLogSearch.addTextChangedListener(object : TextWatcher {
            override fun afterTextChanged(s: Editable?) {
                renderLogs(s?.toString().orEmpty())
            }
            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) {}
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) {}
        })

        btnCopy.setOnClickListener {
            val text = logStore.exportFormattedText()
            val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
            clipboard.setPrimaryClip(ClipData.newPlainText("PCP_Logs", text))
            Toast.makeText(this, "Logs copied to clipboard!", Toast.LENGTH_SHORT).show()
        }

        btnClear.setOnClickListener {
            logStore.clear()
            renderLogs("")
            Toast.makeText(this, "Logs cleared", Toast.LENGTH_SHORT).show()
        }
    }

    private fun renderLogs(query: String) {
        val events = logStore.getAllEvents()
        val filtered = if (query.isBlank()) {
            events
        } else {
            events.filter {
                it.category.contains(query, ignoreCase = true) ||
                it.eventCode.contains(query, ignoreCase = true) ||
                it.message.contains(query, ignoreCase = true) ||
                it.level.contains(query, ignoreCase = true)
            }
        }

        if (filtered.isEmpty()) {
            tvLogsContent.text = "No diagnostic events match query: '$query'"
            return
        }

        val sb = StringBuilder()
        for (e in filtered.reversed()) {
            val colorIndicator = when (e.level) {
                "ERROR" -> "🔴"
                "WARN" -> "🟡"
                "DEBUG" -> "⚪"
                else -> "🟢"
            }
            sb.append("$colorIndicator [${e.timestamp}] [${e.category}] ${e.eventCode}\n   ${e.message}\n\n")
        }
        tvLogsContent.text = sb.toString()
    }
}
