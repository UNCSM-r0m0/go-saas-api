# Claw Memory System - Engram Integration
# Sistema de memoria personal para el agente Claw

import sys
import os
from datetime import datetime

# Agregar engram-memory al path
ENGRAM_PATH = os.path.join(os.path.expanduser("~"), "go-saas-api", "engram-memory")
if ENGRAM_PATH not in sys.path:
    sys.path.insert(0, ENGRAM_PATH)

try:
    from engram import EngramMemory
    ENGRAM_AVAILABLE = True
except ImportError:
    ENGRAM_AVAILABLE = False
    print("⚠️  Engram no disponible - ejecutando sin memoria persistente")

class ClawMemory:
    """
    Sistema de memoria personal de Claw
    Recuerda información sobre r0lm0 y nuestras conversaciones
    """
    
    def __init__(self):
        self.engram = None
        self.user_name = "r0lm0"
        self.session_start = datetime.now()
        
        if ENGRAM_AVAILABLE:
            try:
                self.engram = EngramMemory()
                print("✅ Memoria Engram activada")
            except Exception as e:
                print(f"⚠️  Error inicializando Engram: {e}")
    
    def remember_user_fact(self, fact: str, category: str = "general"):
        """
        Recuerda un hecho sobre el usuario
        
        Categorías: preference, tech_stack, project, learning, personal
        """
        if not self.engram:
            return False
            
        try:
            self.engram.save(
                content=f"[{category.upper()}] {fact}",
                session_id=f"user-{self.user_name}",
                metadata={
                    "type": "user_fact",
                    "category": category,
                    "user": self.user_name,
                    "timestamp": datetime.now().isoformat()
                }
            )
            return True
        except Exception as e:
            print(f"Error guardando memoria: {e}")
            return False
    
    def remember_conversation(self, topic: str, key_points: list):
        """
        Recuerda puntos clave de una conversación
        """
        if not self.engram:
            return False
            
        content = f"Tema: {topic}\n" + "\n".join([f"- {point}" for point in key_points])
        
        try:
            self.engram.save(
                content=content,
                session_id=f"conversation-{datetime.now().strftime('%Y-%m-%d')}",
                metadata={
                    "type": "conversation_summary",
                    "topic": topic,
                    "user": self.user_name
                }
            )
            return True
        except Exception as e:
            return False
    
    def remember_decision(self, decision: str, context: str = ""):
        """
        Recuerda una decisión importante tomada
        """
        if not self.engram:
            return False
            
        content = f"DECISIÓN: {decision}"
        if context:
            content += f"\nContexto: {context}"
            
        try:
            self.engram.save(
                content=content,
                session_id="decisions",
                metadata={
                    "type": "decision",
                    "user": self.user_name,
                    "date": datetime.now().isoformat()
                }
            )
            return True
        except Exception as e:
            return False
    
    def get_user_context(self) -> str:
        """
        Recupera contexto sobre el usuario al iniciar sesión
        """
        if not self.engram:
            return ""
            
        context_parts = []
        
        # Buscar hechos sobre el usuario
        try:
            facts = self.engram.query("user r0lm0", limit=10)
            if facts:
                context_parts.append("## Información sobre r0lm0:")
                for fact in facts[:5]:
                    content = fact['content'].replace('[', '').replace(']', '')
                    context_parts.append(f"- {content}")
        except:
            pass
        
        # Buscar decisiones recientes
        try:
            decisions = self.engram.get_recent(session_id="decisions", limit=5)
            if decisions:
                context_parts.append("\n## Decisiones recientes:")
                for dec in decisions[:3]:
                    content = dec['content'].replace('DECISIÓN:', '').strip()
                    context_parts.append(f"- {content[:100]}...")
        except:
            pass
        
        return "\n".join(context_parts) if context_parts else ""
    
    def recall_related(self, topic: str) -> list:
        """
        Recuerda información relacionada con un tema
        """
        if not self.engram:
            return []
            
        try:
            return self.engram.query(topic, limit=5)
        except:
            return []
    
    def get_recent_context(self, limit: int = 10) -> str:
        """
        Obtiene contexto reciente de conversaciones
        """
        if not self.engram:
            return ""
            
        try:
            recent = self.engram.get_recent(limit=limit)
            if recent:
                return "\n".join([f"- {r['content'][:100]}..." for r in recent[:5]])
        except:
            pass
        
        return ""

# Instancia global
claw_memory = ClawMemory()

# Funciones de conveniencia para usar en respuestas
def remember_user(fact: str, category: str = "general"):
    """Recuerda un hecho sobre el usuario"""
    return claw_memory.remember_user_fact(fact, category)

def remember_topic(topic: str, points: list):
    """Recuerda puntos clave de un tema"""
    return claw_memory.remember_conversation(topic, points)

def remember_decision(decision: str, context: str = ""):
    """Recuerda una decisión importante"""
    return claw_memory.remember_decision(decision, context)

def get_memory_context() -> str:
    """Obtiene contexto de memoria para la conversación actual"""
    return claw_memory.get_user_context()

def recall(topic: str) -> list:
    """Recuerda información relacionada"""
    return claw_memory.recall_related(topic)

# Cargar contexto al iniciar
print("🧠 Cargando memoria de Engram...")
USER_CONTEXT = get_memory_context()
if USER_CONTEXT:
    print("✅ Contexto cargado:")
    print(USER_CONTEXT[:500] + "..." if len(USER_CONTEXT) > 500 else USER_CONTEXT)
else:
    print("ℹ️  No hay contexto previo guardado")
    print("💡 Sugerencia: Empezar a guardar información con remember_user()")
