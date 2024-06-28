package main

import "github.com/ethereum/go-ethereum/common/hexutil"

// Label known rollups who regularly post blob transactions.
var accountLabels = map[[20]byte]string{
	mustDecode("0xc1b634853cb333d3ad8663715b08f41a3aec47cc"): "Arbitrum",
	mustDecode("0x6887246668a3b87f54deb3b94ba47a6f63f32985"): "Optimism",
	mustDecode("0x5050f69a9786f081509234f1a7f4684b5e5b76c9"): "Base",
	mustDecode("0x000000633b68f5d8d3a86593ebb815b4663bcbe0"): "Taiko",
	mustDecode("0x2c169dfe5fbba12957bdd0ba47d9cedbfe260ca7"): "Starknet",
	mustDecode("0x0D3250c3D5FAcb74Ac15834096397a3Ef790ec99"): "ZkSync",
	mustDecode("0xcf2898225ed05be911d3709d9417e86e0b4cfc8f"): "Scroll",
	mustDecode("0x415c8893d514f9bc5211d36eeda4183226b84aa7"): "Blast",
	mustDecode("0xa9268341831efa4937537bc3e9eb36dbece83c7e"): "Linea",
}

func mustDecode(address string) [20]byte {
	byteAddr := hexutil.MustDecode(address)
	return [20]byte(byteAddr)
}
