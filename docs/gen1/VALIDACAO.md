# Validação Gen1

Os testes são do servidor, transporte TCP, proteção da interface nativa e diagnóstico. Os atores de BOTW não foram executados neste ambiente.

## Cobertura automatizada

- Disputa concorrente de coleta entre dois jogadores: somente um vencedor; estado pessoal não vazado ao outro; registro repetido do objeto não ressuscita o item.
- Equipamento/descarte rejeitados quando o item pertence ao outro inventário; descarte seguido de coleta transfere a posse.
- Bootstrap preserva o inventário inicial; identidade de item inicial não colide com novos itens coletados.
- Dano de PvP só na vítima, durabilidade só na arma do atacante, alcance/cooldown e repetição de sequência sem novo dano.
- Dano ambiental em terceiros rejeitado; nenhuma operação de cura arbitrária.
- Monstro troca para o jogador mais próximo; transfere autoridade e geração; ignora pose/ataque de dono antigo e preserva o cooldown durante a troca.
- Monstro não persegue mortos, poses antigas, desconectados ou jogadores em outro mundo.
- Token de reconexão recupera a vida e bolsa do próprio jogador; terceiro cliente/perfil já conectado são recusados; cópias do snapshot não alteram o servidor.
- Duas conexões TCP reais, coleta compartilhada e recuperação após desconexão.
- Runtime incompleto ou simulado não passa como nativo; snapshot não inicializado não limpa a bolsa; estado pessoal de outro destinatário não pode ser aplicado.
- Planos não duplicam o ator local, não criam remoto em outro mapa e ativam IA apenas no dono vigente.
- Diagnóstico usa apenas a lista fixa de consultas e metadados; NSO sintético verifica hash/Build ID sem copiar bytes ao relatório; falta de `hm_caps` é reportada como incompatibilidade.
- Regressão da detecção de porta no arquivo que o launcher realmente captura, uso de `--config`, separação de logs dos clientes 1/2 e log de sandbox.
- Inspeção nativa limitada: módulo desconhecido não dispara leituras; código divergente interrompe a cadeia; ponteiro nulo não é seguido; decoder de vida rejeita chamadas e registradores estranhos. Os testes usam instruções sintéticas próprias, sem bytes extraídos do jogo.

## Comandos

```sh
go test -race -count=1 ./internal/...
go vet ./internal/gen1 ./internal/nativebridge ./cmd/hyrulem-gen1-server ./cmd/hyrulem-native-probe
```

O Actions executa também os testes no Windows, a regressão da interface 0.4.2 e a compilação dos executáveis Windows. Resultados e artefatos ficam associados ao commit da branch, sem publicar uma release Gen1 jogável.

## Testes reais pendentes

Criação e descarte de Link nativo, animação, colisão, eventos reais de acerto/coleta, reconciliação da bolsa e vida, troca do alvo real de IA, estabilidade de GPU/runtime, dois jogos reais simultâneos e LAN/Radmin. O módulo fornecido já foi analisado estaticamente; a inspeção ao vivo e as chamadas de criação ainda precisam de validação. Não há evidência desses itens concluídos.
