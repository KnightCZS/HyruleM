# Auditoria — 0.5.0-preview.14

## Evidência e alteração

O vídeo do usuário, de 15/11, mostra as duas instâncias, chapéu rígido durante salto, sobreposição de roupa/corpo e rastro claro da espada apenas na origem. Não é evidência do resultado da prévia 14.

A busca anterior por armadura exigia o campo de parentesco do ator e os diagnósticos não encontravam ossos de roupa. A nova captura consulta as seis referências do gerenciador de armadura do jogador. Confere geração, resolve acesso temporário pelo motor e libera inclusive quando negado. Identifica a peça pela família do ator, admitindo nomes de armaduras melhoradas que compartilham um modelo.

A paleta de ossos não contém a visibilidade de osso/material. O quadro visual acrescenta máscaras por identidade/nome; o receptor chama os setters nativos apenas para valores alterados. Os índices da origem não são reutilizados.

O hook de nome anterior dependia de um identificador de saída e não cobria chamadas diretas por descritor. Um novo hook captura o Event inicializado e deduplica a API antiga. As condições numéricas são copiadas por nome/tipo aos usuários ELink correspondentes, antes dos pedidos remotos, sem ativar o controlador Player.

## Validação

- C sintético: armadura sem parentesco; rejeição de geração antiga/acesso negado; referência liberada; paletas distintas, isolamento de partes e CPU preservada; máscaras de osso/material e nomes; propriedades reordenadas e setters; pedido por descritor sem saída; duplicatas/NaN/tamanhos recusados.
- Go/race/vet e TCP real do launcher: dois clientes mantêm pose, equipamento, eventos e aparência. O teste de maior quadro usa todos os limites e resulta em 56.594 bytes. Interpolação conserva máscaras/condições no quadro discreto dos eventos; troca de roupa escolhe o quadro completo novo.
- Verificação privada do módulo 1.9.0: 35 assinaturas próprias únicas; setters/leitores de visibilidade, nome de material e setters numéricos de ELink executados com memória sintética. O bit de alteração das propriedades foi conferido.
- Emissão compilada dos hooks, incluindo a instrução STR original do hook novo; funções envolvidas regeneradas/compiladas com e sem registradores locais.
- Build/instalador Windows pelo CI. A publicação não inclui módulos, recursos, saves, código de jogo gerado ou disassembly.

## Limites

Não executado com duas instâncias/GPU nesta sessão. Roupas deformadas exclusivamente por vértices, todos os acessórios, tinturas, condições textuais/instantâneas e renderizadores próprios de rastro não estão integralmente cobertos. Identificador ELink e contador de ossos não provam um efeito desenhado. Esta prévia não implementa combate/IA/interação geral entre Links.

ABI do adaptador e protocolo passam a 10, gamelib permanece 4. É obrigatória regeneração da biblioteca privada com o novo gerador; o pacote não reutiliza a biblioteca das prévias 12/13 como se tivesse o hook novo. A linha estável e a branch main ficam preservadas.
