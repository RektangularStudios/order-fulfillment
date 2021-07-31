import json
import os
import sys

def main():
  # read args
  if len(sys.argv) != 3:
    raise ValueError("expected 2 args: in_path & out_path")
  in_path = sys.argv[1]
  out_path = sys.argv[2]

  with open(in_path, "r") as f:
    utxosJSON = json.load(f)

  utxosSimplifiedJSON = {
    "utxos": []
  }
  # iterate UTXO IDs
  for txid in utxosJSON:
    utxo = {
      "txid": txid,
      "assets": [
        {
          "currency_id": "lovelace",
          "quantity": utxosJSON[txid]["amount"][0],
        },
      ],
    }
    # iterate policy IDs
    for i in range(0, len(utxosJSON[txid]["amount"][1])):
      policy_id = utxosJSON[txid]["amount"][1][i][0]
      # iterate asset ID
      for j in range(0, len(utxosJSON[txid]["amount"][1][i][1])):
        asset_id = utxosJSON[txid]["amount"][1][i][1][j][0]
        quantity = utxosJSON[txid]["amount"][1][i][1][j][1]

        utxo["assets"].append({
          "currency_id": policy_id + "." + asset_id,
          "quantity": quantity,
        })
    
    utxosSimplifiedJSON["utxos"].append(utxo)

  with open(out_path, "w") as f:
    json.dump(utxosSimplifiedJSON, f, indent=2)


if __name__ == "__main__":
  main()
