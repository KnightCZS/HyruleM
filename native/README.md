# HyruleM 0.5.0-preview.6

Os novos relatórios da prévia 5 confirmam criação aceita e falha `remote-unloaded-open-new-session` antes de qualquer movimento/animação. O wake nativo agenda a ativação: a prévia anterior rejeitava o estado transitório 0/2 por exigir estado 1 na mesma chamada. Esta versão mantém a referência retida e aguarda callbacks posteriores, com prazo limitado. O teste privado das rotinas nativas reproduziu essa sequência; a visibilidade em gameplay ainda precisa de confirmação.

## Atualizar e testar

1. Feche os dois jogos e o launcher antigo. Extraia o ZIP inteiro.
2. Execute **Iniciar-Nativo.cmd**. Ele confere/atualiza o runtime configurado, com backup e restauração em falha.
3. Abra ambos os clientes com o pacote novo. O overlay da prévia 5 continua válido, incluindo a variante de contingência. **ABI 3 / protocolo 4** continuam iguais: a biblioteca que já funcionou com ABI 3 pode ser reutilizada.
4. Teste posição, direção e ações nos dois sentidos. O resultado visual desta versão ainda precisa de confirmação em gameplay.

## Recuperação visual e diagnóstico

Se a criação principal for rejeitada, há uma única tentativa de contingência com os recursos originais da configuração anteriormente visível. O menu mostra **Modelo de contingência ativo · movimento sem animação**. Esse ator não recebe chamadas AS, para não executar seletores/eventos do Player. Isso é degradação explícita, não reprodução completa das ações.

O menu distingue **aguardando ativação** de modelo ativo. Estado transitório ou acesso temporariamente indisponível não encerra a sessão no primeiro callback. Uma geração de referência inválida continua sendo erro. A limpeza também aceita um personagem ainda não ativo; se o jogo negar acesso, a referência fica retida até a limpeza ser permitida, sem criar outro personagem. A contingência recebe seu próprio prazo no launcher, sem renovação ilimitada por pacotes repetidos.

Um modelo pronto não é apagado só porque a animação está indisponível. Se as duas variantes falharem, a sessão para e o erro fica preservado. Não há um ciclo ilimitado de criação.

**Diagnosticar.cmd** consulta ambos os clientes em leitura. Os relatórios `native-preview-diag.6` incluem `reference_status`, `actor_state`, `activation_wait_ticks`, `first_error`, `spawn_attempts`, `resource_fallback`, `visual_only`, política/identidade do overlay e mensagens recentes de criação. A presença de uma primeira falha seguida de contingência ativa não significa animação funcionando. `native_ready` permanece reservado à Gen1 completa, ainda incompleta.

As folhas AS agora são arquivos canônicos mínimos, e a ASList tem strings reempacotadas. Seu orçamento acompanha o crescimento, sem reservar 64 KiB adicionais por AS pequeno. A captura e o transporte de ações da prévia 4 permanecem. Clipes representativos ainda não fornecem fidelidade completa de arma/velocidade; cabelo, equipamentos, PvP e IA do mundo não estão integrados.

## O que compartilha

- Vida dos monstros do mapa observados a até 140 m, no máximo 64 por leitura e 128 por sessão.
- Menor vida conhecida pelo servidor, incluindo zero. Não sincroniza ainda animação de morte, movimento ou autoridade da IA.
- Só um monstro carregado com o mesmo hash, nome e posição original recebe o valor. Endereços e IDs de processo nunca viajam pela rede.
- Saves, vida e inventário de cada jogador permanecem separados. O pacote não contém jogo, modelos, saves ou biblioteca recompilada.

Teste no mundo aberto. Missões, baús, itens no chão, árvores, clima, horário, Blood Moon e mudanças de mapa não são sincronizados. Reiniciar o servidor inicia outra sessão de estado compartilhado.

## Validação

- Callback real e armazenamento de HashId conferidos no módulo fornecido; quatorze assinaturas únicas.
- Testes C para controlador/dono, CPU, referências, criação/remoção e aplicação de vida somente na thread do jogo ao monstro correspondente.
- Testes com dois clientes para dano/vida zero convergindo, observações atrasadas sem cura e identidade inválida.
- Recursos derivados privadamente da instalação, preservando registros e subarquivos originais, sem controlador de física/dano do Player no proxy.

Pesquisa: [RESEARCH_SYNC.md](RESEARCH_SYNC.md). Runtime base [Savage Wind](https://github.com/Revivedx/SavageWindRecomp/tree/740b18025fe11ba03c117dbb56668db304b5043e), licença MIT. Nenhum código ou asset do Milk Bar foi incorporado.
