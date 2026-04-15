"""
Engram Integration for OpenClaw/Claw
Sistema de memoria persistente principal

Usage:
    from engram_integration import remember, recall, get_context
    
    # Guardar información importante
    remember("El usuario prefiere PostgreSQL sobre MySQL")
    
    # Buscar información relevante
    results = recall("base de datos")
    
    # Obtener contexto para la conversación actual
    context = get_context()
"""

import sys
import os

# Ensure engram-memory is in path
ENGRAM_PATH = os.path.join(os.path.expanduser("~"), "go-saas-api", "engram-memory")
if ENGRAM_PATH not in sys.path:
    sys.path.insert(0, ENGRAM_PATH)

try:
    from engram import EngramMemory
except ImportError as e:
    print(f"Error cargando Engram: {e}")
    EngramMemory = None

# Global instance
_engram_instance = None

def _get_engram():
    """Get or create Engram instance"""
    global _engram_instance
    if _engram_instance is None and EngramMemory is not None:
        _engram_instance = EngramMemory()
    return _engram_instance

def remember(content: str, category: str = "conversation", metadata: dict = None) -> bool:
    """
    Guarda información en la memoria persistente
    
    Args:
        content: Información a recordar
        category: Categoría (conversation, fact, preference, project)
        metadata: Metadatos adicionales
        
    Returns:
        True si se guardó correctamente
    """
    try:
        engram = _get_engram()
        if engram is None:
            return False
            
        if metadata is None:
            metadata = {}
        
        metadata['category'] = category
        metadata['source'] = 'claw'
        
        session_id = f"claw-{category}"
        engram.save(content, session_id=session_id, metadata=metadata)
        return True
    except Exception as e:
        print(f"Error guardando en Engram: {e}")
        return False

def recall(query: str, limit: int = 5) -> list:
    """
    Recupera información relevante de la memoria
    
    Args:
        query: Término de búsqueda
        limit: Cantidad máxima de resultados
        
    Returns:
        Lista de memorias relevantes
    """
    try:
        engram = _get_engram()
        if engram is None:
            return []
            
        return engram.query(query, limit=limit)
    except Exception as e:
        print(f"Error recuperando de Engram: {e}")
        return []

def get_context(query: str = None, recent_limit: int = 10) -> str:
    """
    Obtiene contexto de conversaciones anteriores
    
    Args:
        query: Consulta específica (opcional)
        recent_limit: Cantidad de memorias recientes si no hay query
        
    Returns:
        String con contexto formateado
    """
    try:
        engram = _get_engram()
        if engram is None:
            return ""
            
        return engram.get_context(query=query, recent_count=recent_limit)
    except Exception as e:
        return ""

def save_fact(fact: str, category: str = "general") -> bool:
    """
    Guarda un hecho importante sobre el usuario o proyecto
    
    Args:
        fact: Hecho a recordar
        category: Categoría (user, project, preference, tech_stack)
        
    Returns:
        True si se guardó correctamente
    """
    return remember(fact, category=f"fact-{category}", metadata={'type': 'fact'})

def save_interaction(user_msg: str, assistant_msg: str, metadata: dict = None) -> bool:
    """
    Guarda una interacción completa usuario-asistente
    
    Args:
        user_msg: Mensaje del usuario
        assistant_msg: Respuesta del asistente
        metadata: Metadatos adicionales
        
    Returns:
        True si se guardó correctamente
    """
    try:
        engram = _get_engram()
        if engram is None:
            return False
            
        engram.save_interaction(user_msg, assistant_msg, metadata)
        return True
    except Exception as e:
        return False

def get_recent_memories(limit: int = 5, category: str = None) -> list:
    """
    Obtiene memorias recientes
    
    Args:
        limit: Cantidad de memorias
        category: Filtrar por categoría (opcional)
        
    Returns:
        Lista de memorias
    """
    try:
        engram = _get_engram()
        if engram is None:
            return []
            
        session_id = f"claw-{category}" if category else None
        return engram.get_recent(session_id=session_id, limit=limit)
    except Exception as e:
        return []

# Auto-save interactions hook
def auto_save_hook(user_message: str, assistant_response: str, **kwargs):
    """
    Hook para auto-guardar interacciones
    Llamar automáticamente después de cada respuesta
    """
    metadata = {
        'timestamp': str(datetime.now()),
        'type': 'auto_saved'
    }
    metadata.update(kwargs)
    
    save_interaction(user_message, assistant_response, metadata)

# Import datetime for the hook
from datetime import datetime

# Initialize on import
print("✓ Engram Memory System initialized")
print(f"  Database: ~/.local/share/engram/claw-memory.db")

if __name__ == "__main__":
    # Test the integration
    print("\nTesting Engram integration...")
    
    # Save a test memory
    result = remember("Test de integración con Engram", category="test")
    print(f"Remember test: {'✓ OK' if result else '✗ FAILED'}")
    
    # Recall test
    results = recall("integración")
    print(f"Recall test: Found {len(results)} results")
    
    # Get context
    ctx = get_context()
    print(f"Context test: {'✓ OK' if ctx else '✗ No context'}")
    
    print("\nEngram is ready to use as primary memory system!")
