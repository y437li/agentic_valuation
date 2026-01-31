import polars as pl
import json
import os
from datetime import datetime

# Configuration
OUTPUT_DIR = "batch_data/transcripts"
BATCH_SIZE = 1000  # Process in chunks if needed, but for now we load one by one

# Updated logic to use direct HTTPS URLs as verified
BASE_URL = "https://huggingface.co/datasets/glopardo/sp500-earnings-transcripts/resolve/main/"
FILES = [
    "data/train-00000-of-00003.parquet",
    "data/train-00001-of-00003.parquet",
    "data/train-00002-of-00003.parquet"
]

def process_and_save():
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    
    # We will aggregate by Ticker to create {TICKER}.json files
    # To avoid memory issues with 33k rows (which is actually small for RAM, but safe is better),
    # we can process file by file.
    
    # We need a way to append to JSON lists safely. 
    # Since 33k rows is small enough (~1GB raw text maybe?), we might be able to load into memory roughly.
    # But let's process file-by-file and merge.
    
    transcript_registry = {}  # ticker -> list of transcripts

    total_count = 0
    
    for file_name in FILES:
        url = BASE_URL + file_name
        print(f"Loading {url}...")
        
        try:
            df = pl.read_parquet(url)
            print(f"Loaded {len(df)} rows.")
            
            # Columns (verified from previous step): 
            # ticker, title, date, content (transcript text)
            # We should map them to our standard schema
            
            # Normalize column names just in case
            df.columns = [c.lower() for c in df.columns]
            
            # Iterate and group
            # Polars is fast, we can convert to python dicts efficiently
            rows = df.to_dicts()
            
            for row in rows:
                ticker = row.get('ticker')
                if not ticker:
                    continue
                
                ticker = ticker.upper().strip()
                
                # Create Transcript Object
                transcript = {
                    "ticker": ticker,
                    "company_name": row.get('title', ''),
                    "date": str(row.get('date', '')), # Convert to string to ensure JSON serializable
                    "content": row.get('transcript') or row.get('text') or row.get('content', ''),
                    "fiscal_quarter": "Unknown", # Dataset might not have this explicit
                    "source": "glopardo/sp500-earnings-transcripts"
                }
                
                if ticker not in transcript_registry:
                    transcript_registry[ticker] = []
                
                transcript_registry[ticker].append(transcript)
                total_count += 1
                
        except Exception as e:
            print(f"Error processing {file_name}: {e}")
            
    print(f"Total transcripts processed: {total_count}")
    print(f"Total unique tickers: {len(transcript_registry)}")
    
    # Save to JSON files
    print("Saving to JSON...")
    for ticker, transcripts in transcript_registry.items():
        # strict filename cleaning
        safe_ticker = "".join([c for c in ticker if c.isalnum()])
        if not safe_ticker:
            continue
            
        file_path = os.path.join(OUTPUT_DIR, f"{safe_ticker}.json")
        
        # Sort by date descending
        transcripts.sort(key=lambda x: x['date'], reverse=True)
        
        with open(file_path, 'w', encoding='utf-8') as f:
            json.dump(transcripts, f, indent=2)
            
    print("Done!")

if __name__ == "__main__":
    process_and_save()
