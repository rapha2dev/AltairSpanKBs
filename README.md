<img src="./altair.png" width="150" height="150">

## Descrição
Altair Spankabytes é um simples interpretador feito exclusivamente para rodar a linguagem criada para a rinha de compiladores. 

## Características
- Baseado em tree-walking interpreter.
- Feito apenas em Go 1.21 com suas libs internas.
- Todas as otimizações são genericas, não importando se é um cálculo de fibonacci, fatorial, etc.
- Interpreta em dois passos:
    1. Pré-Runtime: faz verificações para memoização, configura a construção de escopos e cria funções específicas para executar cada nó da AST.
    2. Runtime: execução recursiva das funções dos nós e verificações de erros.

## Funcionalidades
- [x] Shadowing
- [x] Memoização automática
- [x] Usa int64 como padrão e faz conversão automática em runtime para bigint (suportando valores altos no fibonacci com recursão de cauda).
- [x] Descreve erros em runtime indicando a linha/coluna e o código do trecho problemático.
- [x] Suporta recursões profundas.
- [x] Mecânismo que utiliza goroutines para evitar estouro de pilha, sem alterar o desempenho. (Demanda-se mais memória RAM)

## Desempenho
Intel(R) Core(TM) i5-9600KF CPU @ 3.70GHz

|Exemplo           | Altair           
|:-----------------|:-----------------:|
|fib(46)           |~0.0005 secs       |
|fib(430000)       |~6.1 secs          |
|hanoi(420000)     |~15.0 secs         |
|sum(420000)       |~0.82 secs         |

## Como utilizar
```
go run . ./examples/fib.json
```

## Como testar
Dependências:
```
cargo install rinha
```
Execução dos testes:
```
go test -v ./interpreter
```
ou 
```
go run . ./examples/my-test.rinha
```
