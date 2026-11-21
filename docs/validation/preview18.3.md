# Validação da prévia 18.3

Executada em 21 de novembro de 2026. Dois jogos Switch 1.9.0 rodaram no Windows com GPU, abertos pelo menu do launcher e conectados a um processo independente `HyruleM-Server.exe`. O botão de hospedar do launcher não foi utilizado.

- Commit do pacote: `856be0bbc5be80df20bde1ae83cf08bd43912c4c`.
- Runtime Savage Wind de origem: `740b18025fe11ba03c117dbb56668db304b5043e`.
- Protocolo 13, referências 14, ABI nativa 12.
- ZIP nativo Windows: 25.632.757 bytes, SHA-256 `8920839b93dc028e0558bb489cdd7da3e4988293f28b416e89c690247c772946`.
- [Pipeline nativo](https://github.com/KnightCZS/HyruleM/actions/runs/44944981516) e [build geral](https://github.com/KnightCZS/HyruleM/actions/runs/44944981467) concluídos com sucesso.

## Jogo real

| Verificação | Resultado observado |
| --- | --- |
| Autoridade do servidor | Todos os snapshots observados identificaram `server` como autoridade; os clientes usaram o modo nativo 3. |
| Mesmo inimigo | Bokoblin de mapa `277007734`, vida 13 → 0 nos dois clientes; a leitura da lista nativa confirmou ausência do ator ativo nos dois. Capturas renderizadas mostraram a dissolução. |
| Pinheiro em Rito | `Obj_TreeConiferous_A_02`, identidade `3177583454`, substituído pelo toco nos dois jogos. |
| Árvore larga em outra região | `Obj_TreeBroadleaf_A_L`, identidade `3951367546`, substituída pelo toco nos dois jogos. O segundo cliente executou os golpes. |
| Coleta simultânea | Maçã global 5 ficou coletada nas duas visões. Houve um único recibo nativo de crédito, no cliente 2; o slot nativo local era 1. |
| Distância e retorno | Clientes separados por mais de 2 km durante 22 segundos; a presença voltou a ficar ativa após a reunião. |
| Saída do primeiro participante | O servidor continuou executando e o segundo cliente manteve a integração do mundo. O primeiro reconectou. |
| Reinício do dedicado | Com os jogos ainda abertos, ambos desconectaram, o processo dedicado foi encerrado e reaberto com o mesmo arquivo. Ambos reconectaram com o mesmo UUID de mundo e o item continuou coletado. |
| Arquivo persistido | 62 entidades, 5 itens e 3 registros de vida zero ao final; época do mundo passou de 1 para 2. |
| Transporte | Em 14.052 snapshots, 11.511 usaram referência ao quadro de atores anterior; cerca de 82%. Isso suprimiu 11.965.332 bytes de blobs repetidos de atores, sem contar os demais campos. |

Uma auditoria adicional na prévia 18.2 comparou as coordenadas reais de até 32 NPCs de mapa compartilhados em Rito, em oito amostras. A maior diferença observada foi 0,975 m, com várias diferenças iguais a zero. A captura não valida toda a física, colisões ou todas as ações dos NPCs.

## Regressões encontradas e corrigidas

A prévia 18.1 reconstruiu corretamente a biblioteca privada a partir dos arquivos do proprietário. A primeira compilação real levou 431 segundos; os clientes e as atualizações seguintes reutilizaram a biblioteca compatível.

O teste seguinte encontrou encerramento da integração durante uma demora do controle local ao carregar uma região. A 18.2 passou a recuperar a conexão local no próximo ciclo, sem repetir um comando cujo resultado ficou incerto.

Na 18.2, os dois clientes receberam vida zero do Bokoblin, mas um deles ainda manteve seu ator ativo. A 18.3 retira inimigos mortos dos quadros de movimento e libera a referência nativa que aplicava a pose anterior. O novo teste confirmou o encerramento do ator nos dois jogos.

## Testes automatizados

Passaram Go com detector de races, builds Windows/Linux, testes da interface Windows, instalação com preservação e recuperação, emissão de hooks e C com memória sintética. A suíte cobre transferência de autoridade, rejeição de quadros antigos, isolamento de santuários, 160 itens em cinco regiões, reciclagem dos slots nativos, coleta exclusiva, corrupção do arquivo, falha de gravação e reinício do servidor.

## Limites desta validação

A prévia aceita dois jogadores. O servidor mantém o registro global e delega a simulação nativa a clientes com a entidade carregada. Sem cliente próximo, a entidade conserva o último estado. A janela nativa acompanha até 64 atores em 180 m por cliente e até 32 itens próximos; o registro de itens aceita até 32.768 identidades.

Continuam pendentes a criação geral de atores ausentes, diferenças de escala/tipo de inimigo entre saves, espólios dinâmicos, troncos e madeira, outras variedades de árvore, projéteis, eventos gerais de missões, interações completas e física geral. Materiais de mapa têm coleta exclusiva; a convergência física de todos os materiais móveis ainda precisa de integração. Inventários continuam em saves privados dos clientes. O funcionamento de todos os interiores e de redes entre máquinas distintas não foi validado nesta execução com dois jogos no mesmo PC.

[Executar o servidor dedicado](../DEDICATED_SERVER.md).
