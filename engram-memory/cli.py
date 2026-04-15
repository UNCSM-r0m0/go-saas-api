#!/usr/bin/env python3
"""
Engram Memory System - CLI Tool
Sistema de memoria persistente para Claw (OpenClaw)
Basado en Gentle-AI de Gentleman Programming
"""

import sys
import os
import argparse
import json
from datetime import datetime

# Add parent directory to path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

try:
    from engram import EngramMemory
except ImportError:
    print("Error: No se pudo importar EngramMemory")
    print("Asegúrate de estar en el directorio correcto")
    sys.exit(1)

def main():
    parser = argparse.ArgumentParser(
        description='Engram - Sistema de memoria persistente para Claw'
    )
    
    subparsers = parser.add_subparsers(dest='command', help='Comandos disponibles')
    
    # Save command
    save_parser = subparsers.add_parser('save', help='Guardar una memoria')
    save_parser.add_argument('content', help='Contenido a guardar')
    save_parser.add_argument('--session', '-s', default='default', help='ID de sesión')
    save_parser.add_argument('--meta', '-m', help='Metadatos en formato JSON')
    
    # Query command
    query_parser = subparsers.add_parser('query', help='Buscar memorias')
    query_parser.add_argument('text', help='Texto de búsqueda')
    query_parser.add_argument('--limit', '-l', type=int, default=10, help='Límite de resultados')
    
    # Recent command
    recent_parser = subparsers.add_parser('recent', help='Ver memorias recientes')
    recent_parser.add_argument('--session', '-s', help='Filtrar por sesión')
    recent_parser.add_argument('--limit', '-l', type=int, default=20, help='Límite de resultados')
    
    # Stats command
    subparsers.add_parser('stats', help='Estadísticas de la base de datos')
    
    # Context command
    context_parser = subparsers.add_parser('context', help='Obtener contexto para conversación')
    context_parser.add_argument('--query', '-q', help='Consulta de búsqueda')
    context_parser.add_argument('--limit', '-l', type=int, default=20, help='Cantidad de contexto')
    
    args = parser.parse_args()
    
    if not args.command:
        parser.print_help()
        sys.exit(1)
    
    # Initialize Engram
    engram = EngramMemory()
    
    if args.command == 'save':
        metadata = {}
        if args.meta:
            try:
                metadata = json.loads(args.meta)
            except json.JSONDecodeError:
                print("Error: Metadatos deben ser JSON válido")
                sys.exit(1)
        
        memory_id = engram.save(args.content, session_id=args.session, metadata=metadata)
        print(f"✓ Memoria guardada (ID: {memory_id})")
    
    elif args.command == 'query':
        results = engram.query(args.text, limit=args.limit)
        if not results:
            print("No se encontraron memorias")
        else:
            print(f"Encontradas {len(results)} memorias:\n")
            for mem in results:
                timestamp = mem['timestamp'][:16] if isinstance(mem['timestamp'], str) else mem['timestamp']
                print(f"[{timestamp}] {mem['content'][:100]}...")
                print(f"   Sesión: {mem['session_id']}")
                print()
    
    elif args.command == 'recent':
        results = engram.get_recent(session_id=args.session, limit=args.limit)
        if not results:
            print("No hay memorias recientes")
        else:
            print(f"Últimas {len(results)} memorias:\n")
            for mem in results:
                timestamp = mem['timestamp'][:16] if isinstance(mem['timestamp'], str) else mem['timestamp']
                print(f"[{timestamp}] {mem['content'][:80]}...")
    
    elif args.command == 'stats':
        stats = engram.get_stats()
        print("Estadísticas de Engram:")
        print(f"  Total de memorias: {stats['total_memories']}")
        print(f"  Total de sesiones: {stats['total_sessions']}")
        print(f"  Ubicación: {stats['database_path']}")
    
    elif args.command == 'context':
        context = engram.get_context(query=args.query, recent_count=args.limit)
        if context:
            print(context)
        else:
            print("# No hay contexto previo disponible")

if __name__ == '__main__':
    main()
