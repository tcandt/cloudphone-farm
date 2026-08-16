package com.tcandt.cloudphone.agent.command

import android.content.ContentValues
import android.content.Context
import android.database.sqlite.SQLiteDatabase
import android.database.sqlite.SQLiteOpenHelper

data class JournalRecord(
    val commandId: String,
    val fencingToken: Long,
    val status: String,
    val error: String?,
    val executedAt: Long
)

class CommandJournal(context: Context) : SQLiteOpenHelper(context, DATABASE_NAME, null, DATABASE_VERSION) {

    companion object {
        private const val DATABASE_NAME = "pcp_command_journal.db"
        private const val DATABASE_VERSION = 1

        private const val TABLE_JOURNAL = "command_journal"
        private const val COL_COMMAND_ID = "command_id"
        private const val COL_FENCING_TOKEN = "fencing_token"
        private const val COL_STATUS = "status"
        private const val COL_ERROR = "error"
        private const val COL_EXECUTED_AT = "executed_at"
    }

    override fun onCreate(db: SQLiteDatabase) {
        val createSql = """
            CREATE TABLE IF NOT EXISTS $TABLE_JOURNAL (
                $COL_COMMAND_ID TEXT PRIMARY KEY,
                $COL_FENCING_TOKEN INTEGER NOT NULL,
                $COL_STATUS TEXT NOT NULL,
                $COL_ERROR TEXT,
                $COL_EXECUTED_AT INTEGER NOT NULL
            );
        """.trimIndent()
        db.execSQL(createSql)
    }

    override fun onUpgrade(db: SQLiteDatabase, oldVersion: Int, newVersion: Int) {
        db.execSQL("DROP TABLE IF EXISTS $TABLE_JOURNAL")
        onCreate(db)
    }

    @Synchronized
    fun getRecord(commandId: String): JournalRecord? {
        return try {
            val db = readableDatabase
            val cursor = db.query(
                TABLE_JOURNAL,
                arrayOf(COL_COMMAND_ID, COL_FENCING_TOKEN, COL_STATUS, COL_ERROR, COL_EXECUTED_AT),
                "$COL_COMMAND_ID = ?",
                arrayOf(commandId),
                null, null, null
            )

            cursor.use {
                if (it.moveToFirst()) {
                    JournalRecord(
                        commandId = it.getString(it.getColumnIndexOrThrow(COL_COMMAND_ID)),
                        fencingToken = it.getLong(it.getColumnIndexOrThrow(COL_FENCING_TOKEN)),
                        status = it.getString(it.getColumnIndexOrThrow(COL_STATUS)),
                        error = it.getString(it.getColumnIndexOrThrow(COL_ERROR)),
                        executedAt = it.getLong(it.getColumnIndexOrThrow(COL_EXECUTED_AT))
                    )
                } else null
            }
        } catch (e: Throwable) {
            null
        }
    }

    @Synchronized
    fun saveRecord(commandId: String, fencingToken: Long, status: String, error: String?): JournalRecord {
        val record = JournalRecord(
            commandId = commandId,
            fencingToken = fencingToken,
            status = status,
            error = error,
            executedAt = System.currentTimeMillis()
        )

        try {
            val db = writableDatabase
            val values = ContentValues().apply {
                put(COL_COMMAND_ID, commandId)
                put(COL_FENCING_TOKEN, fencingToken)
                put(COL_STATUS, status)
                put(COL_ERROR, error)
                put(COL_EXECUTED_AT, record.executedAt)
            }

            db.insertWithOnConflict(TABLE_JOURNAL, null, values, SQLiteDatabase.CONFLICT_REPLACE)
        } catch (e: Throwable) {
            // Ignored in JVM test environment
        }
        return record
    }
}
