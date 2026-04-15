# Engram Integration for Claw
# Sistema de memoria persistente principal

import os
import json
import sqlite3
from datetime import datetime
from typing import Dict, List, Optional, Any

class EngramMemory:
    """
    Sistema de memoria persistente para Claw
    Basado en el ecosistema Gentle-AI
    """
    
    def __init__(self, db_path: str = None):
        if db_path is None:
            home = os.path.expanduser("~")
            db_path = os.path.join(home, ".local", "share", "engram", "claw-memory.db")
        
        self.db_path = db_path
        self._init_database()
    
    def _init_database(self):
        """Inicializa la base de datos SQLite con FTS5"""
        os.makedirs(os.path.dirname(self.db_path), exist_ok=True)
        
        conn = sqlite3.connect(self.db_path)
        cursor = conn.cursor()
        
        # Tabla principal de memorias
        cursor.execute("""
            CREATE TABLE IF NOT EXISTS memories (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                session_id TEXT NOT NULL,
                content TEXT NOT NULL,
                metadata TEXT,
                timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
            )
        """)
        
        # Tabla virtual FTS5 para búsqueda
        cursor.execute("""
            CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
                content, metadata, content_rowid=rowid
            )
        """)
        
        # Tabla de sesiones
        cursor.execute("""
            CREATE TABLE IF NOT EXISTS sessions (
                id TEXT PRIMARY KEY,
                title TEXT,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
            )
        """)
        
        # Triggers para mantener FTS indexado
        cursor.execute("""
            CREATE TRIGGER IF NOT EXISTS memories_insert_fts 
            AFTER INSERT ON memories BEGIN
                INSERT INTO memories_fts(rowid, content, metadata) 
                VALUES (new.id, new.content, new.metadata);
            END
        """)
        
        cursor.execute("""
            CREATE TRIGGER IF NOT EXISTS memories_delete_fts 
            AFTER DELETE ON memories BEGIN
                INSERT INTO memories_fts(memories_fts, rowid, content, metadata) 
                VALUES ('delete', old.id, old.content, old.metadata);
            END
        """)
        
        # Índices
        cursor.execute("CREATE INDEX IF NOT EXISTS idx_session ON memories(session_id)")
        cursor.execute("CREATE INDEX IF NOT EXISTS idx_timestamp ON memories(timestamp)")
        
        conn.commit()
        conn.close()
    
    def save(self, content: str, session_id: str = "default", 
             metadata: Dict[str, Any] = None) -> int:
        """
        Guarda una nueva memoria
        
        Args:
            content: Contenido de la memoria
            session_id: ID de sesión (default: "default")
            metadata: Metadatos adicionales
            
        Returns:
            ID de la memoria guardada
        """
        if metadata is None:
            metadata = {}
        
        metadata["source"] = "claw"
        metadata["saved_at"] = datetime.now().isoformat()
        
        conn = sqlite3.connect(self.db_path)
        cursor = conn.cursor()
        
        # Asegurar que la sesión existe
        cursor.execute(
            "INSERT OR IGNORE INTO sessions (id) VALUES (?)",
            (session_id,)
        )
        
        # Insertar memoria
        cursor.execute(
            "INSERT INTO memories (session_id, content, metadata) VALUES (?, ?, ?)",
            (session_id, content, json.dumps(metadata))
        )
        
        memory_id = cursor.lastrowid
        
        # Actualizar timestamp de sesión
        cursor.execute(
            "UPDATE sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = ?",
            (session_id,)
        )
        
        conn.commit()
        conn.close()
        
        return memory_id
    
    def query(self, query_text: str, limit: int = 10) -> List[Dict[str, Any]]:
        """
        Busca memorias usando FTS5
        
        Args:
            query_text: Texto de búsqueda
            limit: Límite de resultados
            
        Returns:
            Lista de memorias encontradas
        """
        conn = sqlite3.connect(self.db_path)
        cursor = conn.cursor()
        
        cursor.execute("""
            SELECT m.id, m.session_id, m.content, m.metadata, m.timestamp
            FROM memories m
            JOIN memories_fts fts ON m.id = fts.rowid
            WHERE memories_fts MATCH ?
            ORDER BY rank
            LIMIT ?
        """, (query_text, limit))
        
        results = []
        for row in cursor.fetchall():
            metadata = json.loads(row[3]) if row[3] else {}
            results.append({
                "id": row[0],
                "session_id": row[1],
                "content": row[2],
                "metadata": metadata,
                "timestamp": row[4]
            })
        
        conn.close()
        return results
    
    def get_recent(self, session_id: str = None, limit: int = 50) -> List[Dict[str, Any]]:
        """
        Obtiene memorias recientes
        
        Args:
            session_id: Filtrar por sesión (None = todas)
            limit: Cantidad de resultados
            
        Returns:
            Lista de memorias ordenadas por fecha
        """
        conn = sqlite3.connect(self.db_path)
        cursor = conn.cursor()
        
        if session_id:
            cursor.execute("""
                SELECT id, session_id, content, metadata, timestamp
                FROM memories
                WHERE session_id = ?
                ORDER BY timestamp DESC
                LIMIT ?
            """, (session_id, limit))
        else:
            cursor.execute("""
                SELECT id, session_id, content, metadata, timestamp
                FROM memories
                ORDER BY timestamp DESC
                LIMIT ?
            """, (limit,))
        
        results = []
        for row in cursor.fetchall():
            metadata = json.loads(row[3]) if row[3] else {}
            results.append({
                "id": row[0],
                "session_id": row[1],
                "content": row[2][:200] + "..." if len(row[2]) > 200 else row[2],
                "metadata": metadata,
                "timestamp": row[4]
            })
        
        conn.close()
        return results
    
    def get_context(self, query: str = None, recent_count: int = 20) -> str:
        """
        Obtiene contexto relevante para la conversación actual
        
        Args:
            query: Consulta de búsqueda (opcional)
            recent_count: Cantidad de memorias recientes si no hay query
            
        Returns:
            String con contexto formateado
        """
        if query:
            memories = self.query(query, limit=10)
        else:
            memories = self.get_recent(limit=recent_count)
        
        if not memories:
            return ""
        
        context_parts = ["## Contexto de conversaciones anteriores:"]
        
        for mem in memories[:5]:  # Solo las 5 más relevantes
            context_parts.append(f"- {mem['content'][:150]}...")
        
        return "\n".join(context_parts)
    
    def save_interaction(self, user_message: str, assistant_response: str, 
                        metadata: Dict[str, Any] = None):
        """
        Guarda una interacción completa usuario-asistente
        
        Args:
            user_message: Mensaje del usuario
            assistant_response: Respuesta del asistente
            metadata: Metadatos adicionales
        """
        if metadata is None:
            metadata = {}
        
        metadata["type"] = "interaction"
        metadata["user_message"] = user_message[:100]  # Preview
        
        content = f"Usuario: {user_message}\nAsistente: {assistant_response}"
        
        self.save(content, metadata=metadata)
    
    def save_fact(self, fact: str, category: str = "general"):
        """
        Guarda un hecho importante sobre el usuario/proyecto
        
        Args:
            fact: Hecho a recordar
            category: Categoría (user, project, preference, etc.)
        """
        self.save(
            content=fact,
            session_id=f"facts-{category}",
            metadata={"type": "fact", "category": category}
        )
    
    def get_stats(self) -> Dict[str, Any]:
        """Obtiene estadísticas de la base de datos"""
        conn = sqlite3.connect(self.db_path)
        cursor = conn.cursor()
        
        cursor.execute("SELECT COUNT(*) FROM memories")
        total_memories = cursor.fetchone()[0]
        
        cursor.execute("SELECT COUNT(*) FROM sessions")
        total_sessions = cursor.fetchone()[0]
        
        cursor.execute("""
            SELECT session_id, COUNT(*) as count 
            FROM memories 
            GROUP BY session_id 
            ORDER BY count DESC 
            LIMIT 5
        """)
        top_sessions = cursor.fetchall()
        
        conn.close()
        
        return {
            "total_memories": total_memories,
            "total_sessions": total_sessions,
            "top_sessions": top_sessions,
            "database_path": self.db_path
        }

# Instancia global para uso de Claw
engram = EngramMemory()
